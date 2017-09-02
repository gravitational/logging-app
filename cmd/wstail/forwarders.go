package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/logging-app/lib/forwarders"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

func upsertForwarder(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	var forwarder forwarders.Forwarder

	err := readJSON(r, &forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	err = writeForwarder(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	err = reload()
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("upserted log forwarder: %v", forwarder)
	return nil
}

func deleteForwarder(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	name := p.ByName("name")
	if name == "" {
		return trace.BadParameter("log forwarder name is missing")
	}

	err := os.Remove(path(name))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	err = reload()
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("log forwarder %q removed", name)
	return nil
}

// replaceForwarders updates log forwarder configuration and reloads the logging service.
// It receives new forwarder configuration, updates the configuration and restarts the rsyslog daemon
// to force it to reload the configuration
func replaceForwarders(w http.ResponseWriter, r *http.Request, p httprouter.Params) (err error) {
	var forwarders []forwarders.Forwarder
	if err = readJSON(r, &forwarders); err != nil {
		return trace.Wrap(err)
	}

	// Remove previous forwarder configuration files
	if err = filepath.Walk(rsyslogConfigDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if path == rsyslogConfigDir {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		log.Infof("removing %v", path)
		return trace.Wrap(os.Remove(path), "failed to remove %v", path)
	}); err != nil {
		log.Warningf("failed to delete forwarder configuration files: %v", err)
	}

	// Write new forwarder configuration
	for _, forwarder := range forwarders {
		if err := writeForwarder(forwarder); err != nil {
			return trace.Wrap(err)
		}
	}

	// Reload rsyslogd
	err = reload()
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("log forwarder configuration replaced")
	return nil
}

func writeForwarder(f forwarders.Forwarder) error {
	file, err := os.Create(path(f.Addr))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()
	_, err = file.WriteString(config(f))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func reload() error {
	out, err := exec.Command(rsyslogInitScript, "restart").CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "failed to restart rsyslogd: %s", out)
	}
	return nil
}

const rsyslogConfigDir = "/etc/rsyslog.d"

const rsyslogInitScript = "/etc/init.d/rsyslog"

func config(forwarder forwarders.Forwarder) string {
	return fmt.Sprintf("*.* %v%v", protocol(forwarder), forwarder.Addr)
}

func path(addr string) string {
	name := fmt.Sprintf("%v.conf", strings.Replace(addr, ":", "_", -1))
	return filepath.Join(rsyslogConfigDir, name)
}

func protocol(forwarder forwarders.Forwarder) string {
	switch forwarder.Protocol {
	case "udp":
		return "@"
	case "tcp":
		return "@@"
	default:
		return "@@"
	}
}

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
