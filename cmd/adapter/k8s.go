/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"github.com/jrivets/log4g"
	"github.com/logrange/logrange/client"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type (
	kCli struct {
		cfg    *Config
		cli    *kubernetes.Clientset
		logger log4g.Logger
	}

	grForwarderCfg struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Address  string `yaml:"address"`
			Protocol string `yaml:"protocol,omitempty"`
		} `yaml:"spec"`
	}

	lrFwdPatch struct {
		Data struct {
			ForwardJson string `json:"forward.json"`
		} `json:"data"`
	}
)

const (
	lrCfgMapFwdKey = "forward.json"
)

func newKCli(cfg *Config) (*kCli, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed getting K8s config, err=%v", err)
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed creating K8s client, err=%v", err)
	}

	logger := log4g.GetLogger("kcli")
	return &kCli{cfg: cfg, cli: cli, logger: logger}, nil
}

func (k *kCli) getLrFwdCfg() (*client.Config, error) {
	cfgMap, err := k.cli.CoreV1().
		ConfigMaps(k.cfg.Logrange.Kubernetes.Namespace).
		Get(k.cfg.Logrange.Kubernetes.ForwarderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	lFwdKey := lrCfgMapFwdKey
	lCfgFwdStr, ok := cfgMap.Data[lFwdKey]
	if !ok {
		return nil, fmt.Errorf("no key=%v found in configMap=%v",
			lFwdKey, str(cfgMap))
	}

	var lCfg client.Config
	if err := json.Unmarshal([]byte(lCfgFwdStr), &lCfg); err != nil {
		k.logger.Error("Data=", lCfgFwdStr, ", err=", err)
		return nil, fmt.Errorf("failed unmarshal key=%v from configMap=%v",
			lFwdKey, str(cfgMap))
	}

	return &lCfg, err
}

func (k *kCli) getGrFwdCfg() ([]*grForwarderCfg, error) {
	cfgMap, err := k.cli.CoreV1().
		ConfigMaps(k.cfg.Gravity.Kubernetes.Namespace).
		Get(k.cfg.Gravity.Kubernetes.ForwarderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	grFwdCfgs := make([]*grForwarderCfg, 0, len(cfgMap.Data))
	for key, data := range cfgMap.Data {
		var grCfg grForwarderCfg
		if err := yaml.Unmarshal([]byte(data), &grCfg); err != nil {
			k.logger.Error("Data=", data, ", err=", err)
			k.logger.Warn("failed unmarshal key=%v from configMap=%v, skipping key...", key, str(cfgMap))
			continue
		}
		grFwdCfgs = append(grFwdCfgs, &grCfg)
	}

	return grFwdCfgs, err
}

func (k *kCli) updateLrFwdCfg(cfg *client.Config) error {
	bb, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var patch lrFwdPatch
	patch.Data.ForwardJson = string(bb)

	bb, err = json.Marshal(patch)
	if err == nil {
		_, err = k.cli.CoreV1().
			ConfigMaps(k.cfg.Logrange.Kubernetes.Namespace).
			Patch(k.cfg.Logrange.Kubernetes.ForwarderConfigMapName, types.StrategicMergePatchType, bb)
	}
	return err
}

func str(cfgMap *v1.ConfigMap) string {
	if cfgMap == nil {
		return ""
	}
	return fmt.Sprintf("%v/%v", cfgMap.Namespace, cfgMap.Name)
}
