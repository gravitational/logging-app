package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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

// tailHistory defines the number of lines to output
const tailHistory = 100

func main() {
	log.SetLevel(log.InfoLevel)
	flag.Parse()
	if flag.NArg() < 1 {
		filePath = defaultTailSource
	} else {
		filePath = flag.Args()[0]
	}

	http.Handle("/ws", makeHandler(serveWs))

	log.Infof("listening on %v", *addr)
	log.Fatalln(http.ListenAndServe(*addr, nil))
}

func writer(ws *websocket.Conn, filter filter) {
	matcher := buildMatcher(filter)
	log.Infof("active filter: %v (%v)", filter, matcher)
	defer ws.Close()

	var err error
	var history io.ReadCloser
	tailCmd := exec.Command("tail", "-f", filePath)
	commands := []*exec.Cmd{tailCmd}
	if matcher != "" {
		history, err = snapshot(matcher, filePath, tailHistory)
		if err != nil {
			log.Warningf("failed to obtain history for %v: %v", matcher, trace.DebugReport(err))
		}
		// --line-buffered is not supported in busybox
		// grepCmd := exec.Command("grep", "--line-buffered", "-E", matcher)
		grepCmd := exec.Command("grep", "-E", matcher)
		commands = append(commands, grepCmd)
	}
	pipe, err := pipeCommands(commands...)
	if err != nil {
		log.Errorf("failed to build a command pipeline: %v", err)
		return
	}
	defer pipe.Close()

	messageC := newMessagePump(pipe, history)
	closeNotifierC := newCloseNotifierLoop(ws)

	var errDisconnected error
	for errDisconnected == nil {
		select {
		case <-closeNotifierC:
			log.Infof("client disconnected")
			return
		case line := <-messageC:
			var payload = struct {
				Type    string `json:"type"`
				Payload string `json:"payload"`
			}{
				Type:    "data",
				Payload: string(line),
			}

			if data, err := json.Marshal(&payload); err != nil {
				log.Infof("failed to convert to JSON: %v", err)
			} else {
				errDisconnected = ws.WriteMessage(websocket.TextMessage, data)
			}
		}
	}
}

// newCloseNotifierLoop spawns a goroutine that reads from the client.
// The loop terminates once a message from the client has been received.
// The only expected message the close message although the goroutine does not validate this fact.
// Returns a channel that will be closed if client disconnects.
func newCloseNotifierLoop(ws *websocket.Conn) chan struct{} {
	notifierC := make(chan struct{})
	go func() {
		var err error
		for err == nil {
			var msg []byte
			_, msg, err = ws.ReadMessage()
			if err == nil {
				log.Infof("recv: %s", msg)
			}
		}
		log.Infof("closing connection with %v", err)
		close(notifierC)
	}()
	return notifierC
}

// newMessagePump spawns a goroutine to handle messages from the tailing process group.
// Returns a channel where the received messages are sent to.
func newMessagePump(r io.Reader, history io.ReadCloser) chan []byte {
	// Log message format:
	// <timestamp> <log-forwader-pod> <kubernetes-logfile-reference> <JSON-encoded-log-message>
	// Since the output of this loop is the log message alone, skip this many
	// columns to only output the relevant detail
	const columnsToSkip = 3

	messageC := make(chan []byte)
	go func() {
		if history != nil {
			defer history.Close()
			r = io.MultiReader(history, r)
		}

		s := bufio.NewScanner(r)
		s.Split(bufio.ScanLines)
		for s.Scan() {
			line := s.Bytes()

			// Skip to the actual log message in the stream
			var logEntryPos int
			for i := 0; i < columnsToSkip; i++ {
				logEntryPos = bytes.IndexByte(line, ' ')
				if logEntryPos >= 0 {
					line = line[logEntryPos+1:]
				}
			}

			messageC <- line
		}
		log.Infof("closing tail message pump: %v", s.Err())
	}()
	return messageC
}

// snapshot takes a snapshot of up to tailHistory last lines of logging history
// for the specified matcher
func snapshot(matcher, filePath string, tailHistory int) (io.ReadCloser, error) {
	log.Infof("requesting history for %v", matcher)
	grepCmd := exec.Command("grep", "-E", matcher, filePath)
	tailCmd := exec.Command("tail", fmt.Sprintf("-n%v", tailHistory))
	pipe, err := pipeCommands(grepCmd, tailCmd)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pipe, nil
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
	return n, err
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
		filter.freeText = string(data)
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

var (
	filePath string
	addr     = flag.String("addr", ":8083", "websocket service address")
)
