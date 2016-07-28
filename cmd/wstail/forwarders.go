package main

import (
	"net/http"
	"strings"

	"github.com/gravitational/trace"
)

// updateForwarders updates log forwarder configuration and reloads the logging
// service.
// It receives new forwarder configuration, updates the ConfigMap and sends
// a SIGHUP to rsyslog as a request to reload the configuration
func updateForwarders(w http.ResponseWriter, r *http.Request) (err error) {
	var forwarders forwarderConfig
	if err = readJSON(r, &forwarders); err != nil {
		return trace.Wrap(err)
	}

	log.Infof("forwarder configuration update: %v", forwarders)

	f, err := ioutil.TempFile("/tmp", "configmap")
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	if err = configTemplate.Execute(f, forwarders); err != nil {
		return trace.Wrap(err)
	}

	if out, err = kubectlCmd("apply", "-f", f.Name()); err != nil {
		return trace.Wrap(err, "failed to apply ConfigMap:\n%s", out)
	}

	var namespace string
	if namespace = os.Getenv("POD_NAMESPACE"); namespace == "" {
		namespace = "kube-system"
	}

	namespaceFlag := fmt.Sprintf("--namespace=", namespace)
	// Restart the log collector by deleting the Pod
	if out, err = kubectlCmd("get", "po", namespaceFlag, "-l=role=log-collector",
		`--output=jsonpath='{range .items[*]}{.metadata.name}{","}{end}'`); err != nil {
		return trace.Wrap(err, "failed to find log collector pods: %s", out)
	}
	podNames := bytes.Split(out, []byte(','))
	for _, podName := range podNames {
		podName = bytes.TrimSpace(podName)
		if out, err = kubectlCmd("delete", "po", string(podName), namespaceFlag); err != nil {
			return trace.Wrap(err, "failed to delete pod %s:\n%s", podName, out)
		}
	}
	// TODO: wait until replication controller has restarted the pod
	// kubectl get rc -l=name=log-collector -o jsonpath={.items[*].status.replicas}
	if err = utils.RetryWithAbort(retryInterval, retryAttempts, func() (bool, error) {
		out, err = kubectlCmd("get", "rc", "-l=name=log-collector", namespaceFlag,
			"{.items[*].status.replicas}")
		if err != nil {
			return false, trace.Wrap(err, "failed to query rc status: %s", out)
		}
		if !bytes.Equal(bytes.TrimSpace(out), '1') {
			log.Infof("rc status invalid, expected replicas=1, got `%s`", out)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, trace.Wrap(err, "failed to wait on rc completion condition")
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
	Hoststring `json:"host_port"`
	// Protocol defines the protocol to configure for this forwarder (TCP/UDP)
	Protocol string `json:"protocol"`
}

func forwarderName(forwader forwarder) string {
	return strings.Replace(forwarder.HostPort, ":", "_", -1)
}

func forwarderProtocol(forwader forwarder) string {
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
  namespace: kube-system
data:
  {{range .Forwarders -}}
  {{- forwarderName .}}.conf: *.* {{forwarderProtocol .}}{{.HostPort}}
  {{end -}}
`))
