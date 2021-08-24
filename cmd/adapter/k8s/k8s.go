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

package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/gravitational/logrus"
	"github.com/gravitational/trace"
	"github.com/logrange/logrange/client"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/mohae/deepcopy"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type (
	// Wrapper around standard k8s API client with encapsulated
	// configs in order have info for building domain specific high level
	// operations (like sync forwarders) on top of common low level
	// operations provided by standard k8s API client
	Client struct {
		// Gravity k8s config
		gravityCfg *Config
		// Logrange k8s config
		lograngeCfg *Config
		// Logrange forwarder default config template
		lograngeFwdTmpl *forwarder.WorkerConfig
		// Standard k8s api client
		cli *kubernetes.Clientset

		logger *log.Entry
		ctx    context.Context
	}

	// Represents Gravity or Logrange k8s config
	Config struct {
		// k8s namespace
		Namespace string
		// k8s forwarders configMap in the given namespace
		ForwarderConfigMapName string
	}

	// Represents Gravity forwarders k8s configMap
	gravityForwarderCfg struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Address  string `yaml:"address"`
			Protocol string `yaml:"protocol,omitempty"`
		} `yaml:"spec"`
	}

	// Represents Logrange forwarders k8s configMap,
	// this part is patched during Gravity to Logrange sync
	lograngeFwdPatch struct {
		Data struct {
			ForwardJson string `json:"forward.json"`
		} `json:"data"`
	}
)

const (
	// Key that is updated during Gravity to Logrange sync
	lrCfgMapFwdKey = "forward.json"
)

// Creates new domain specific K8s client for the given configs
func NewClient(ctx context.Context, gravityK8sCfg *Config, lograngeK8sCfg *Config,
	lograngeFwdTmpl *forwarder.WorkerConfig) (*Client, error) {

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed getting K8s config")
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed creating K8s client")
	}
	return &Client{
		gravityCfg:      gravityK8sCfg,
		lograngeCfg:     lograngeK8sCfg,
		lograngeFwdTmpl: lograngeFwdTmpl,
		cli:             cli,
		logger:          log.WithField(trace.Component, "logging-app.k8s"),
		ctx:             ctx,
	}, nil
}

// Syncs forwarders configuration from Gravity configMap to Logrange configMap.
// The operation is needed since in Gravity cluster, forwarders configuration
// is stored in its own place and has its own format
// (and we can't break backward compatibility for now), at the same time Logrange
// has different place to store this info and slightly different format...
func (cli *Client) SyncForwarders(ctx context.Context) {
	cli.logger.Debug("sync(): Getting Logrange forwarder config...")
	lrFwdCfg, err := cli.getLograngeForwarderCfg()
	if err != nil {
		cli.logger.Error("sync(): Err=", err)
		return
	}

	cli.logger.Debug("sync(): Getting Gravity forwarder config...")
	grFwdCfgs, err := cli.getGravityForwarderConfig()
	if err != nil {
		cli.logger.Error("sync(): Err=", err)
		return
	}

	cli.logger.Debug("sync(): Filtering invalid configs...")
	grFwdCfgs = cli.filterInvalidCfgs(grFwdCfgs)

	cli.logger.Debug("sync(): Merging forwarder configs...")
	newFwdCfg, err := cli.mergeFwdConfigs(lrFwdCfg.Forwarder, grFwdCfgs, cli.lograngeFwdTmpl)
	if err != nil {
		cli.logger.Error("sync(): Err=", err)
		return
	}

	cli.logger.Info("sync(): Updating Logrange forwarder config: from=", lrFwdCfg.Forwarder, " to=", newFwdCfg)
	lrFwdCfg.Forwarder = newFwdCfg
	if err = cli.updateLograngeFwdCfg(lrFwdCfg); err != nil {
		cli.logger.Error("sync(): Err=", err)
	}
}

// Filters Gravity forwarder configs which can't be translated
// to Logrange forwarder config (due to absence of required info)
func (cli *Client) filterInvalidCfgs(grFwdCfgs []*gravityForwarderCfg) []*gravityForwarderCfg {
	filteredCfgs := make([]*gravityForwarderCfg, 0, len(grFwdCfgs))
	for _, grCfg := range grFwdCfgs {
		if grCfg.Metadata.Name == "" {
			cli.logger.Warn("filter(): 'name' is empty in Gravity cfg=", grCfg, ", skipping cfg...")
			continue
		}
		if grCfg.Spec.Address == "" {
			cli.logger.Warn("filter(): 'address' is empty in Gravity cfg=", grCfg, ", skipping cfg...")
			continue
		}
		filteredCfgs = append(filteredCfgs, grCfg)
	}
	return filteredCfgs
}

// Merges Gravity forwarder configs into Logrange forwarder config,
// all default values (e.g. protocol) are taken from template wCfgTmpl (WorkerConfig)
func (cli *Client) mergeFwdConfigs(lrFwdCfg *forwarder.Config, grFwdCfgs []*gravityForwarderCfg,
	wCfgTmpl *forwarder.WorkerConfig) (*forwarder.Config, error) {

	// copy	passed in Logrange forwarder config, to replace forwarder workers
	// with whatever is currently in Gravity forwarder config
	lrFwdCfg = deepcopy.Copy(lrFwdCfg).(*forwarder.Config)
	lrFwdCfg.Workers = make([]*forwarder.WorkerConfig, 0, len(grFwdCfgs))

	// run through Gravity forwarder configs and transform them to Logrange
	// forwarder configs, append the result to Logrange forwarder workers
	for _, grCfg := range grFwdCfgs {
		wCfg := deepcopy.Copy(wCfgTmpl).(*forwarder.WorkerConfig)
		wCfg.Name = grCfg.Metadata.Name
		wCfg.Sink.Params["RemoteAddr"] = grCfg.Spec.Address
		if grCfg.Spec.Protocol != "" {
			wCfg.Sink.Params["Protocol"] = grCfg.Spec.Protocol
		}
		lrFwdCfg.Workers = append(lrFwdCfg.Workers, wCfg)
	}

	// return updated Logrange config
	return lrFwdCfg, nil
}

func (cli *Client) getLograngeForwarderCfg() (*client.Config, error) {
	cfgMap, err := cli.cli.CoreV1().
		ConfigMaps(cli.lograngeCfg.Namespace).
		Get(cli.ctx, cli.lograngeCfg.ForwarderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lFwdKey := lrCfgMapFwdKey
	lCfgFwdStr, ok := cfgMap.Data[lFwdKey]
	if !ok {
		return nil, trace.NotFound("no key=%v found in configMap=%v",
			lFwdKey, str(cfgMap))
	}

	var lCfg client.Config
	if err := json.Unmarshal([]byte(lCfgFwdStr), &lCfg); err != nil {
		cli.logger.Error("Data=", lCfgFwdStr, ", err=", err)
		return nil, trace.Errorf("failed unmarshal key=%v from configMap=%v",
			lFwdKey, str(cfgMap))
	}

	return &lCfg, nil
}

func (cli *Client) getGravityForwarderConfig() ([]*gravityForwarderCfg, error) {
	cfgMap, err := cli.cli.CoreV1().
		ConfigMaps(cli.gravityCfg.Namespace).
		Get(cli.ctx, cli.gravityCfg.ForwarderConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grFwdCfgs := make([]*gravityForwarderCfg, 0, len(cfgMap.Data))
	for key, data := range cfgMap.Data {
		var grCfg gravityForwarderCfg
		if err := yaml.Unmarshal([]byte(data), &grCfg); err != nil {
			cli.logger.Error("Data=", data, ", err=", err)
			cli.logger.Warn("Failed unmarshal key=", key, " from configMap=", str(cfgMap), ", skipping key...")
			continue
		}
		grFwdCfgs = append(grFwdCfgs, &grCfg)
	}

	return grFwdCfgs, nil
}

// Patches Logrange forwarder config
func (cli *Client) updateLograngeFwdCfg(cfg *client.Config) error {
	patchBytes, err := json.Marshal(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	var patch lograngeFwdPatch
	patch.Data.ForwardJson = string(patchBytes)

	patchBytes, err = json.Marshal(patch)
	if err == nil {
		_, err = cli.cli.CoreV1().
			ConfigMaps(cli.lograngeCfg.Namespace).
			Patch(cli.ctx, cli.lograngeCfg.ForwarderConfigMapName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	return trace.Wrap(err)
}

// Merges current config with the given one
func (cfg *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	if other.Namespace != "" {
		cfg.Namespace = other.Namespace
	}
	if other.ForwarderConfigMapName != "" {
		cfg.ForwarderConfigMapName = other.ForwarderConfigMapName
	}
}

// Checks whether current config is valid and safe to use
func (cfg *Config) Check() error {
	if cfg.Namespace == "" {
		return trace.BadParameter("invalid Namespace: must be non-empty")
	}
	if cfg.ForwarderConfigMapName == "" {
		return trace.BadParameter("invalid ForwarderConfigMapName: must be non-empty")
	}
	return nil
}

func (cfg *Config) String() string {
	return utils.ToJsonStr(cfg)
}

func str(cfgMap *v1.ConfigMap) string {
	if cfgMap == nil {
		return ""
	}
	return fmt.Sprintf("%v/%v", cfgMap.Namespace, cfgMap.Name)
}
