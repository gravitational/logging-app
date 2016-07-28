package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

// updateForwarders updates log forwarder configuration and reloads the logging
// service.
// It receives new forwarder configuration, updates the ConfigMap and restarts the collector
// pod as a request to reload the rsyslogd configuration
func updateForwarders(w http.ResponseWriter, r *http.Request) (err error) {
	if r.Method != "PUT" {
		return trace.BadParameter("invalid HTTP method: %v", r.Method)
	}
	var forwarders []forwarder
	if err = readJSON(r, &forwarders); err != nil {
		return trace.Wrap(err)
	}

	var namespace string
	if namespace = os.Getenv("POD_NAMESPACE"); namespace == "" {
		namespace = "kube-system"
	}

	log.Infof("forwarder configuration update: %v", forwarders)

	f, err := ioutil.TempFile("/tmp", "configmap")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	var config = struct {
		Namespace  string
		Forwarders []forwarder
	}{
		Namespace:  namespace,
		Forwarders: forwarders,
	}
	if err = configTemplate.Execute(io.MultiWriter(os.Stdout, f), &config); err != nil {
		return trace.Wrap(err)
	}

	var out []byte
	if out, err = kubectlCmd("apply", "-f", f.Name()); err != nil {
		log.Errorf("failed to apply ConfigMap:\n%s", out)
		return trace.Wrap(err, "failed to apply ConfigMap:\n%s", out)
	}

	namespaceFlag := fmt.Sprintf("--namespace=%v", namespace)
	// Restart the log collector by deleting the collector pod
	if out, err = kubectlCmd("get", "po", namespaceFlag, "-l=role=log-collector",
		`--output=jsonpath={range .items[*]}{.metadata.name}{","}{end}`); err != nil {
		return trace.Wrap(err, "failed to find log collector pods: %s", out)
	}
	podNames := bytes.Split(out, []byte{','})
	for _, podName := range podNames {
		podName = bytes.TrimSpace(podName)
		if len(podName) == 0 {
			continue
		}
		if out, err = kubectlCmd("delete", "po", string(podName), namespaceFlag); err != nil {
			return trace.Wrap(err, "failed to delete pod %s:\n%s", podName, out)
		}
	}
	if err = retryWithAbort(retryInterval, retryAttempts, func() (bool, error) {
		out, err = kubectlCmd("get", "rc", "-l=name=log-collector", namespaceFlag,
			"--output=jsonpath={.items[*].status.replicas}")
		if err != nil {
			return false, trace.Wrap(err, "failed to query rc status: %s", out)
		}
		if !bytes.Equal(bytes.TrimSpace(out), []byte{'1'}) {
			log.Infof("rc status invalid, expected replicas=1, got `%s`", out)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return trace.Wrap(err, "failed to wait on rc completion condition")
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

func forwarderName(forwarder forwarder) string {
	return strings.Replace(forwarder.HostPort, ":", "", -1)
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

var forwarderFuncs = template.FuncMap{
	"forwarderName":     forwarderName,
	"forwarderProtocol": forwarderProtocol,
}

var configTemplate = template.Must(template.New("forwarder").Funcs(forwarderFuncs).Parse(`
kind: ConfigMap
apiVersion: v1
metadata:
  name: extra-log-collector-config
  namespace: {{ .Namespace }}
data:{{range .Forwarders}}
  {{forwarderName .}}.conf: "*.* {{forwarderProtocol .}}{{.HostPort}}"{{end}}
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

const retryInterval = 5 * time.Second
const retryAttempts = 50

func retryWithAbort(period time.Duration, attempts int, fn func() (bool, error)) (err error) {
	for i := 1; i <= attempts; i += 1 {
		var succeeded bool
		if succeeded, err = fn(); succeeded {
			return nil
		} else if err == nil {
			log.Infof("unsuccessfull attempt:%v, retry in %v", trace.UserMessage(err), period)
			time.Sleep(period)
			continue
		}
		break
	}
	log.Errorf("all attempts failed:\n%v", trace.DebugReport(err))
	return trace.Wrap(err)
}
