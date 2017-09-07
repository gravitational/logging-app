package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// initLogForwarders reads config map that contains log forwarder resources and creates
// respective rsyslog configuration files
func initLogForwarders() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	configMap, err := client.ConfigMaps(systemNamespace).Get(
		forwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(configMap.Data) == 0 {
		log.Infof("no log forwarders configured")
		return nil
	}

	for _, data := range configMap.Data {
		err := initLogForwarder([]byte(data))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// initLogForwarders configures a single log forwarder from data found in config map
func initLogForwarder(data []byte) error {
	var forwarder logForwarder

	err := yaml.Unmarshal([]byte(data), &forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(
		forwarderFilename(forwarder),
		forwarderConfig(forwarder),
		0755)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("configured log forwarder %v", forwarder)
	return nil
}

// forwarderFilename returns a full path to log forwarder config file
func forwarderFilename(forwarder logForwarder) string {
	return filepath.Join(rsyslogConfigDir, forwarder.Metadata.Name)
}

// forwarderConfig returns log forwarder rsyslog config
func forwarderConfig(forwarder logForwarder) []byte {
	if forwarder.Spec.Protocol == "udp" {
		return []byte(fmt.Sprintf("*.* @%v", forwarder.Spec.Address))
	}
	return []byte(fmt.Sprintf("*.* @@%v", forwarder.Spec.Address))
}

// logForwarder is the log forwarder spec
type logForwarder struct {
	// Metadata is log forwarder metadata
	Metadata struct {
		// Name is log forwarder name
		Name string `json:"name" yaml:"name"`
	} `json:"metadata" yaml:"metadata"`
	// Spec defines log forwarder specification
	Spec struct {
		// Address is forwarding address
		Address string `json:"address" yaml:"address"`
		// Protocol is forwarding protocol
		Protocol string `json:"protocol" yaml:"protocol"`
	} `json:"spec" yaml:"spec"`
}

const (
	// systemNamespace is the Kubernetes system namespace
	systemNamespace = "kube-system"
	// forwardersConfigMap is the name of config map with forwarders
	forwardersConfigMap = "log-forwarders"
	// rsyslogConfigDir is the directory where forwarder configs are put
	rsyslogConfigDir = "/etc/rsyslog.d"
)
