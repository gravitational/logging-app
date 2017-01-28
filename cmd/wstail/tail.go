package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

const defaultTailSource = "/var/log/messages"

// defaultTailLimit defines how many last lines will tail output by default
const defaultTailLimit = 100

// maxDumpLen defines the maximum size of the string window to output
// to console after a failed interpretation (as in failure to decode JSON)
const maxDumpLen = 128

// rotatedLogUncompressed names the first uncompressed (potentially in use)
// rotated log file as named by savelog
const rotatedLogUncompressed = "messages.0"

func logReader(filePath string, filter filter, limit string) ([]byte, error) {
	dir := filepath.Dir(defaultTailSource)
	names, err := readDir(dir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read log directory: %v")
	}

	_, err = os.Stat(filePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	rotated := newRotatedLogs(dir, names)
	log.Infof("rotated logs: %#v", rotated)
	if rotated.Main != "" {
		filePath = fmt.Sprintf("%v %v", rotated.Main, filePath)
	}

	if limit == "" {
		limit = strconv.Itoa(defaultTailLimit)
	}

	var history io.ReadCloser
	var commands []*exec.Cmd
	if filter.isEmpty() {
		commands = []*exec.Cmd{
			exec.Command("tail", "--lines", limit, filePath),
		}
	} else {
		matcher := buildMatcher(filter)
		log.Infof("active filter: %v (%v)", filter, matcher)
		history, err = snapshot(matcher, rotated, -1)
		log.Infof("history pipeline: %s", history)
		if err != nil {
			log.Warningf("failed to obtain history for %v: %v", matcher, trace.DebugReport(err))
		}
		commands = []*exec.Cmd{
			exec.Command("grep", "--line-buffered", "--extended-regexp", matcher, filePath),
			exec.Command("tail", "--lines", limit),
		}
	}

	pipe, err := newProcessGroup(commands...)
	log.Infof("tailing pipeline: %s", pipe)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build a command pipeline")
	}
	defer pipe.Close()

	var messages []string
	ch := makeOutputChannel(pipe, history)
	for message := range ch {
		var dockerMessage dockerLogMessage
		if len(message) > 0 && message[0] == '{' {
			if err = json.Unmarshal([]byte(message), &dockerMessage); err == nil {
				message = dockerMessage.Log
			} else {
				truncAt := len(message)
				if truncAt > maxDumpLen {
					truncAt = maxDumpLen
				}
				log.Infof("failed to unmarshal `%v...`: %v", message[:truncAt], err)
				// Use the message as-is
			}
		}
		var payload = struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}{
			Type:    "data",
			Payload: message,
		}

		data, err := json.Marshal(&payload)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		messages = append(messages, string(data))
	}

	out, err := json.Marshal(messages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// dockerLogMessage defines a partial view of the docker message as received from
// the aggregated log file storage
type dockerLogMessage struct {
	// Log defines the contents of a log message
	Log string `json:"log"`
}

// makeOutputChannel spawns a goroutine to handle messages from the process group.
// Returns a channel where the received messages are sent to.
func makeOutputChannel(r io.Reader, history io.ReadCloser) chan string {
	// Log message format:
	//
	// Kubernetes context (files forwarded from /var/log/containers):
	// <timestamp> <log-forwader-pod> <kubernetes-logfile-reference> <JSON-encoded-log-message>
	// Arbitrary log files:
	// <timestamp> <log-forwader-pod> <filename>.log <text>
	// Since the output of this loop is the log message alone, skip this many
	// columns to only output the relevant detail
	const columnsToSkip = 3

	ch := make(chan string)
	go func() {
		if history != nil {
			r = io.MultiReader(&autoClosingReader{history}, r)
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

			// Convert to string to force a copy of the data as scanner.Bytes()
			// returns a reference to the internal reusable memory buffer
			// TODO: use a pool of reusable slices
			ch <- string(line)
		}
		err := s.Err()
		if err != nil {
			log.Error(trace.Wrap(err))
		}

		close(ch)
	}()
	return ch
}

// autoClosingReader closes the underlined reader when it reaches the end of stream
type autoClosingReader struct {
	io.ReadCloser
}

// Read implements io.Reader
func (r *autoClosingReader) Read(p []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(p)
	if err == io.EOF {
		r.ReadCloser.Close()
	}
	return n, err
}

// snapshot takes a snapshot of history for the specified matcher
// using rotated as input and limits to tailLimit lines of output.
// With tailLimit == -1, everything is output
func snapshot(matcher string, rotated rotatedLogs, tailLimit int) (io.ReadCloser, error) {
	if len(rotated.Compressed) == 0 {
		return nil, nil
	}
	args := append([]string{"--line-buffered", "--no-filename", "-E", matcher}, rotated.Compressed...)
	log.Infof("requesting history for %v", matcher)
	commands := []*exec.Cmd{exec.Command("zgrep", args...)}
	if tailLimit > 0 {
		commands = append(commands, exec.Command("tail", "-n", fmt.Sprintf("%v", tailLimit)))
	}
	pipe, err := newProcessGroup(commands...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pipe, nil
}

func newProcessGroup(commands ...*exec.Cmd) (group *processGroup, err error) {
	var stdout io.ReadCloser
	var closers []io.Closer
	for i, cmd := range commands {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		closers = append(closers, stdout)
		cmd.Start()
		if i < len(commands)-1 {
			commands[i+1].Stdin = stdout
		}
	}

	return &processGroup{
		commands: commands,
		closers:  closers,
		stream:   stdout,
	}, nil
}

// processGroup groups the processes that build a processing pipe
type processGroup struct {
	commands []*exec.Cmd
	closers  []io.Closer
	stream   io.Reader
}

func (r *processGroup) Read(p []byte) (n int, err error) {
	n, err = r.stream.Read(p)
	return n, err
}

func (r *processGroup) Close() (err error) {
	// Close all open stdout handles
	for _, closer := range r.closers {
		closer.Close()
	}
	r.terminate()
	return trace.Wrap(err)
}

func (r *processGroup) String() string {
	var cmds []string
	for _, cmd := range r.commands {
		cmds = append(cmds, fmt.Sprintf("%v", cmd.Args))
	}
	return fmt.Sprintf("[%v]", strings.Join(cmds, ","))
}

// processTerminateTimeout defines the initial amount of time to wait for process to terminate
const processTerminateTimeout = 200 * time.Millisecond

func (r *processGroup) terminate() {
	terminated := make(chan struct{})
	head := r.commands[0]
	go func() {
		for _, cmd := range r.commands {
			// Await termination of all processes in the group to prevent zombie processes
			if err := cmd.Wait(); err != nil {
				log.Infof("%v exited with %v", cmd.Path, err)
			}
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

// getLogs serves /v1/log?query=hello&limit=100
//
// it dumps all logs filtered by query and cutted to limit in the configured filePath
// if none of query params was set, dumps file with default limit tailMaxDepth
func getLogs(w http.ResponseWriter, r *http.Request) (err error) {
	filePath := r.Context().Value(filePathContextKey).(string)
	query := r.URL.Query().Get("query")
	filter, _ := parseQuery([]byte(query))
	limit := r.URL.Query().Get("limit")

	resp, err := logReader(filePath, filter, limit)
	if err != nil {
		return trace.Wrap(err)
	}
	w.Write(resp)
	return nil
}

// downloadLogs serves /v1/download
//
// it creates a gzipped tarball with all logs found in the configured filePath
func downloadLogs(w http.ResponseWriter, r *http.Request) error {
	filePath := r.Context().Value(filePathContextKey).(string)
	dir, file := filepath.Split(filePath)

	gzWriter := gzip.NewWriter(w)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !strings.HasPrefix(info.Name(), file) {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		fileBytes, err := ioutil.ReadFile(path)
		if err != nil {
			return trace.Wrap(err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return trace.Wrap(err)
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = tarWriter.Write(fileBytes)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	w.Header().Set("Content-Disposition", "attachment; filename=logs.tar.gz")
	return trace.Wrap(err)
}

// readDir reads the contents of the directory with the log file(s)
func readDir(dir string) (names []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read directory `%v`", dir)
	}
	names, err = f.Readdirnames(-1)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read directory `%v`", dir)
	}
	return names, nil
}
