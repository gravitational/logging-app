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

func setupLogForwarders() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	configMap, err := client.ConfigMaps("kube-system").Get("log-forwarders", metav1.GetOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(configMap.Data) == 0 {
		log.Infof("no log forwarders configured")
		return nil
	}

	for _, data := range configMap.Data {
		var lf logForwarder
		err := yaml.Unmarshal(data, &lf)
		if err != nil {
			return trace.Wrap(err)
		}

		filename := filepath.Join("/etc/rsyslog.d", lf.Metadata.Name)
		var config string
		if lf.Spec.Protocol == "udp" {
			config = fmt.Sprintf("*.* @%v", lf.Spec.Address)
		} else {
			config = fmt.Sprintf("*.* @@%v", lf.Spec.Address)
		}

		err = ioutil.WriteFile(filename, []byte(config), 0755)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Infof("configured log forwarder %v", lf)
	}

	return nil
}

type logForwarder struct {
	Metadata struct {
		Name string `json:"name" yaml:"name"`
	} `json:"metadata" yaml:"metadata"`
	Spec struct {
		Address  string `json:"address" yaml:"address"`
		Protocol string `json:"address" yaml:"address"`
	} `json:"spec" yaml:"spec"`
}
