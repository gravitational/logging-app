package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)

	var filePath string
	var httpAddr = flag.String("addr", ":8083", "HTTP service address")
	var initForwarders = flag.Bool("init-forwarders", false, "Initialize log forwarders and exit")

	flag.Parse()

	if *initForwarders {
		err := setupLogForwarders()
		if err != nil {
			log.Fatalf("failed to setup log forwarders: %v", trace.DebugReport(err))
		}
		return
	}

	if flag.NArg() < 1 {
		filePath = defaultTailSource
	} else {
		filePath = flag.Args()[0]
	}

	log.Infof("HTTP service listening on %s", *httpAddr)

	router := httprouter.New()
	router.GET("/v1/log", makeHandlerWithFilePath(filePath, getLogs))
	router.GET("/v1/download", makeHandlerWithFilePath(filePath, downloadLogs))

	errChan := make(chan error, 10)
	go func() {
		errChan <- http.ListenAndServe(*httpAddr, router)
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

type handlerWithFilePath func(filePath string, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

func makeHandlerWithFilePath(filePath string, handler handlerWithFilePath) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		err := handler(filePath, w, r, p)
		if err != nil {
			trace.WriteError(w, err)
		}
	}
}
