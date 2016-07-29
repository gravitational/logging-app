package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

// updateForwarders updates log forwarder configuration and reloads the logging
// service.
// It receives new forwarder configuration, updates the configuration and restarts the rsyslog daemon
// to force it to reload the configuration
func updateForwarders(w http.ResponseWriter, r *http.Request) (err error) {
	if r.Method != "PUT" {
		return trace.BadParameter("invalid HTTP method: %v", r.Method)
	}
	var forwarders []forwarder
	if err = readJSON(r, &forwarders); err != nil {
		return trace.Wrap(err)
	}

	// TODO: remove pervious configuration files
	for _, forwarder := range forwarders {
		f, err := os.Create(forwarderPath(forwarder))
		if err != nil {
			return trace.Wrap(err)
		}
		err = configTemplate.Execute(io.MultiWriter(os.Stdout, f), forwarder)
		f.Close()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Reload rsyslogd
	if out, err := exec.Command("/etc/init.d/rsyslog", "restart").CombinedOutput(); err != nil {
		return trace.Wrap(err, "failed to restart rsyslogd: %s", out)
	}

	log.Infof("forwarder configuration updated")
	return nil
}

type forwarderConfig struct {
	Forwarders []forwarder `json:"forwarders"`
}

// forwarder defines a log forwarder
type forwarder struct {
	// HostPort defines the address the forwarder is listening on
	HostPort string `json:"host_port"`
	// Protocol defines the protocol to configure for this forwarder (TCP/UDP)
	Protocol string `json:"protocol"`
}

func forwarderPath(forwarder forwarder) string {
	name := fmt.Sprintf("%v.conf", strings.Replace(forwarder.HostPort, ":", "_", -1))
	return filepath.Join("/etc/rsyslog.d", name)
}

func forwarderProtocol(forwarder forwarder) string {
	switch forwarder.Protocol {
	case "udp":
		return "@"
	case "tcp":
		return "@@"
	default:
		return "@@"
	}
}

var forwarderFuncs = template.FuncMap{"protocol": forwarderProtocol}

var configTemplate = template.Must(template.New("forwarder").Funcs(forwarderFuncs).
	Parse(`*.* {{protocol .}}{{.HostPort}}
`))

func readJSON(r *http.Request, data interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return trace.BadParameter("invalid request: %v", err)
	}
	return nil
}
