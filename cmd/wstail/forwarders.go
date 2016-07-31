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

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

// updateForwarders updates log forwarder configuration and reloads the logging service.
// It receives new forwarder configuration, updates the configuration and restarts the rsyslog daemon
// to force it to reload the configuration
func updateForwarders(w http.ResponseWriter, r *http.Request) (err error) {
	if r.Method != "PUT" {
		return trace.BadParameter("invalid HTTP method: %v", r.Method)
	}
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
		path := path(forwarder)
		f, err := os.Create(path)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = f.WriteString(config(forwarder))
		if errClose := f.Close(); errClose != nil {
			log.Warningf("failed to close file %v: %v", path, errClose)
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Reload rsyslogd
	if out, err := exec.Command(rsyslogInitScript, "restart").CombinedOutput(); err != nil {
		return trace.Wrap(err, "failed to restart rsyslogd: %s", out)
	}

	log.Infof("forwarder configuration updated")
	return nil
}

const rsyslogConfigDir = "/etc/rsyslog.d"

const rsyslogInitScript = "/etc/init.d/rsyslog"

func config(forwarder forwarders.Forwarder) string {
	return fmt.Sprintf("*.* %v%v", protocol(forwarder), forwarder.Addr)
}

func path(forwarder forwarders.Forwarder) string {
	name := fmt.Sprintf("%v.conf", strings.Replace(forwarder.Addr, ":", "_", -1))
	return filepath.Join("/etc/rsyslog.d", name)
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
