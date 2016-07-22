package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"net/http"
	"os/exec"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
)

const defaultTailSource = "/var/log/messages"

func main() {
	log.SetLevel(log.InfoLevel)
	flag.Parse()
	if flag.NArg() < 1 {
		filePath = defaultTailSource
	} else {
		filePath = flag.Args()[0]
	}

	http.Handle("/ws", makeHandler(serveWs))

	log.Fatalln(http.ListenAndServe(*addr, nil))
}

func writer(ws *websocket.Conn, filter filter) {
	log.Infof("active filter: %v", filter)
	defer ws.Close()

	tailCmd := exec.Command("tail", "-f", filePath)
	grepCmd := exec.Command("grep", "--line-buffered", "-E", buildMatcher(filter))
	pipe, err := pipeCommands(tailCmd, grepCmd)
	if err != nil {
		log.Errorf("failed to build command pipeline: %v", err)
	}
	defer pipe.Close()

	s := bufio.NewScanner(pipe)
	s.Split(bufio.ScanLines)
	var errDisconnected error
	for s.Scan() && errDisconnected == nil {
		line := s.Bytes()

		// Skip to the actual log message in the stream
		var logEntryPos int
		for i := 0; i < 3; i++ {
			logEntryPos = bytes.IndexByte(line, ' ')
			if logEntryPos >= 0 {
				line = line[logEntryPos+1:]
			}
		}

		errDisconnected = ws.WriteMessage(websocket.TextMessage, line)
	}
}

func pipeCommands(commands ...*exec.Cmd) (group *processGroup, err error) {
	var stdout io.ReadCloser
	for i, cmd := range commands {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cmd.Start()
		if i < len(commands)-1 {
			commands[i+1].Stdin = stdout
		}
	}

	return &processGroup{
		commands: commands,
		stream:   stdout,
	}, nil
}

type processGroup struct {
	commands []*exec.Cmd
	stream   io.ReadCloser
}

func (r *processGroup) Read(p []byte) (n int, err error) {
	n, err = r.stream.Read(p)
	return n, trace.Wrap(err)
}

func (r *processGroup) Close() (err error) {
	err = r.stream.Close()
	r.terminate()
	return trace.Wrap(err)
}

// processTerminateTimeout defines the initial amount of time to wait for process to terminate
const processTerminateTimeout = 200 * time.Millisecond

func (r *processGroup) terminate() {
	terminated := make(chan struct{})
	head := r.commands[0]
	go func() {
		for _, cmd := range r.commands {
			cmd.Wait()
		}
		terminated <- struct{}{}
	}()

	if err := head.Process.Signal(syscall.SIGINT); err != nil {
		log.Infof("cannot terminate with SIGINT: %v", err)
	}

	select {
	case <-terminated:
		return
	case <-time.After(processTerminateTimeout):
	}

	if err := head.Process.Signal(syscall.SIGTERM); err != nil {
		log.Infof("cannot terminate with SIGTERM: %v", err)
	}

	select {
	case <-terminated:
		return
	case <-time.After(processTerminateTimeout * 2):
		head.Process.Kill()
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) (err error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return trace.Wrap(err, "failed to upgrade to websocket protocol")
	}

	_, data, err := ws.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	filter, err := parseQuery(bytes.NewReader(data))
	if err != nil {
		log.Infof("unable to parse query %s: %v", data, err)
		// TODO: use the filter as raw search string if not in structured form
		return trace.Wrap(err)
	}

	go writer(ws, filter)
	return nil
}

// makeHandler wraps a handler with http.Handler
func makeHandler(handler handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err != nil {
			trace.WriteError(w, err)
		}
	}
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const pollInterval = 2 * time.Second

var (
	filePath string
	addr     = flag.String("addr", ":8083", "websocket service address")
)
