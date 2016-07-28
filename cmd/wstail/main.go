package main

import (
	"flag"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

func main() {
	log.SetLevel(log.InfoLevel)
	flag.Parse()
	if flag.NArg() < 1 {
		filePath = defaultTailSource
	} else {
		filePath = flag.Args()[0]
	}

	http.Handle("/ws", makeHandler(serveWs))
	http.Handle("/forwarders", makeHandler(updateForwarders))

	log.Infof("listening on %v", *addr)
	log.Fatalln(http.ListenAndServe(*addr, nil))
}

var (
	filePath string
	addr     = flag.String("addr", ":8083", "websocket service address")
)

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
