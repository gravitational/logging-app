package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

const filePathContextKey = "filepath"

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

	http.Handle("/v1/log", withContext(filePathContextKey, filePath, makeHandler(getLogs)))
	http.Handle("/v1/download", withContext(filePathContextKey, filePath, makeHandler(downloadLogs)))
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
				log.Fatal(err)
			}
		case s := <-signalChan:
			log.Infof("Captured %v. Exiting...", s)
			os.Exit(0)
		}
	}
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

// withContext is a middleware which allows to put data to request context
func withContext(key, value interface{}, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), key, value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
