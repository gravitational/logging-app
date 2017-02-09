package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

func main() {
	log.SetLevel(log.InfoLevel)

	var (
		filePath string
		httpAddr = flag.String("addr", ":8083", "HTTP service address")
	)
	flag.Parse()
	if flag.NArg() < 1 {
		filePath = defaultTailSource
	} else {
		filePath = flag.Args()[0]
	}

	log.Infof("HTTP service listening on %s", *httpAddr)

	http.Handle("/v1/log", makeHandlerWithFilePath(filePath, getLogs))
	http.Handle("/v1/download", makeHandlerWithFilePath(filePath, downloadLogs))
	http.Handle("/v1/forwarders", makeHandler(updateForwarders))

	errChan := make(chan error, 10)
	go func() {
		errChan <- http.ListenAndServe(*httpAddr, nil)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Ignore()
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case err := <-errChan:
			if err != nil {
				trace.DebugReport(err)
			}
		case s := <-signalChan:
			log.Infof("Captured %v. Exiting...", s)
			return
		}
	}
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

type handlerWithFilePath func(filePath string, w http.ResponseWriter, r *http.Request) error

// makeHandler wraps a handler with http.Handler
func makeHandler(handler handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err != nil {
			trace.WriteError(w, err)
		}
	}
}

func makeHandlerWithFilePath(filePath string, handler handlerWithFilePath) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(filePath, w, r)
		if err != nil {
			trace.WriteError(w, err)
		}
	}
}
