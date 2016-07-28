package main

import (
	"flag"
	"net/http"

	log "github.com/Sirupsen/logrus"
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
