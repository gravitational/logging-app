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
	"github.com/gravitational/logging-app/cmd/adapter/k8s"
	"github.com/gravitational/trace"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/logrange/range/pkg/transport"
	"io/ioutil"
)

type (

	// Represents Gravity configuration
	gravity struct {
		// Address on which http server listens
		ApiListenAddr string
		// K8s config
		Kubernetes *k8s.Config
	}

	// Describe a query, which should be called periodically
	cronQuery struct {
		// Query contains LQL which should be called periodically
		Query string
		// PeriodSec defines an interval in seconds to execute the Query
		PeriodSec int
	}

	// Logrange configuration
	logrange struct {
		// List of queries to be executed periodically
		CronQueries []cronQuery
		// Partition against which LQL (Logrange query language) queries to be executed
		Partition string
		// Path to default/template Logrange forwarder config file
		ForwarderTmplFile string
		// K8s config
		Kubernetes *k8s.Config
		// Defines client <-> server transport config (remote addr, tls, 2waytls, etc)
		Transport *transport.Config
	}

	// Config joins together Gravity and Logrange parts
	// together plus specifies configs sync interval.
	// Fields are exported since this is JSON (un-)marshaled object.
	Config struct {
		Gravity         *gravity
		Logrange        *logrange
		SyncIntervalSec int
	}
)

// Loads adapter config from the given file
func LoadCfgFromFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg := &Config{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

// Creates adapter config with default values
func NewDefaultConfig() *Config {
	return &Config{
		Gravity:         newDefaultGravityConfig(),
		Logrange:        newDefaultLograngeConfig(),
		SyncIntervalSec: 20,
	}
}

// Merges current config with the given one
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	if other.Gravity != nil {
		c.Gravity.merge(other.Gravity)
	}
	if other.Logrange != nil {
		c.Logrange.merge(other.Logrange)
	}
	if other.SyncIntervalSec != 0 {
		c.SyncIntervalSec = other.SyncIntervalSec
	}
}

// Checks whether current config is valid and safe to use
func (c *Config) Check() error {
	if c.Gravity == nil {
		return trace.BadParameter("invalid Gravity: must be non-nil")
	}
	if c.Logrange == nil {
		return trace.BadParameter("invalid Logrange: must be non-nil")
	}
	if err := c.Gravity.check(); err != nil {
		return trace.BadParameter("invalid Gravity=%v: %v", c.Gravity, err)
	}
	if err := c.Logrange.check(); err != nil {
		return trace.BadParameter("invalid Logrange=%v: %v", c.Logrange, err)
	}
	if c.SyncIntervalSec <= 0 {
		return trace.BadParameter("invalid SyncIntervalSec=%v: must be > 0sec", c.SyncIntervalSec)
	}
	return nil
}

func (c *Config) String() string {
	return utils.ToJsonStr(c)
}

func newDefaultGravityConfig() *gravity {
	return &gravity{
		ApiListenAddr: "127.0.0.1:8083",
		Kubernetes: &k8s.Config{
			Namespace:              "kube-system",
			ForwarderConfigMapName: "log-forwarders",
		},
	}
}

func (g *gravity) merge(other *gravity) {
	if other == nil {
		return
	}

	if other.Kubernetes != nil {
		g.Kubernetes.Merge(other.Kubernetes)
	}
	if other.ApiListenAddr != "" {
		g.ApiListenAddr = other.ApiListenAddr
	}
}

func (g *gravity) check() error {
	if g.Kubernetes == nil {
		return trace.BadParameter("invalid Kubernetes: must be non-nil")
	}
	if err := g.Kubernetes.Check(); err != nil {
		return trace.BadParameter("invalid Kubernetes=%v: %v", g.Kubernetes, err)
	}
	if g.ApiListenAddr == "" {
		return trace.BadParameter("invalid ApiListenAddr: must be non-empty")
	}
	return nil
}

func (g *gravity) String() string {
	return utils.ToJsonStr(g)
}

func newDefaultLograngeConfig() *logrange {
	return &logrange{
		Partition:         "logrange.pipe=__default__",
		ForwarderTmplFile: "/opt/logrange/gravity/config/forward-tmpl.json",
		Kubernetes: &k8s.Config{
			Namespace:              "kube-system",
			ForwarderConfigMapName: "lr-forwarder",
		},
		Transport: &transport.Config{
			ListenAddr: "127.0.0.1:9966",
		},
	}
}

func (l *logrange) merge(other *logrange) {
	if other == nil {
		return
	}

	if other.Transport != nil {
		l.Transport.Apply(other.Transport)
	}
	if other.Kubernetes != nil {
		l.Kubernetes.Merge(other.Kubernetes)
	}
	if other.ForwarderTmplFile != "" {
		l.ForwarderTmplFile = other.ForwarderTmplFile
	}
	if other.Partition != "" {
		l.Partition = other.Partition
	}
	if len(other.CronQueries) > 0 {
		l.CronQueries = other.CronQueries
	}
}

func (l *logrange) check() error {
	if l.Kubernetes == nil {
		return trace.BadParameter("invalid Kubernetes: must be non-nil")
	}
	if err := l.Kubernetes.Check(); err != nil {
		return trace.BadParameter("invalid Kubernetes=%v: %v", l.Kubernetes, err)
	}
	if l.Transport == nil {
		return trace.BadParameter("invalid Transport: must be non-nil")
	}
	if err := l.Transport.Check(); err != nil {
		return trace.BadParameter("invalid Transport=%v: %v", l.Transport, err)
	}
	if l.ForwarderTmplFile == "" {
		return trace.BadParameter("invalid ForwarderTmplFile: must be non-empty")
	}
	if _, err := l.getForwarderTmpl(); err != nil {
		return trace.BadParameter("invalid ForwarderTmplFile=%v: %v", l.ForwarderTmplFile, err)
	}
	if l.Partition == "" {
		return trace.BadParameter("invalid Partition: must be non-empty")
	}
	return nil
}

func (l *logrange) getForwarderTmpl() (*forwarder.WorkerConfig, error) {
	buf, err := ioutil.ReadFile(l.ForwarderTmplFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wCfgTmpl := &forwarder.WorkerConfig{}
	if err = json.Unmarshal(buf, wCfgTmpl); err != nil {
		return nil, trace.Wrap(err)
	}
	return wCfgTmpl, nil
}

func (l *logrange) String() string {
	return utils.ToJsonStr(l)
}

func (cq cronQuery) String() string {
	return fmt.Sprintf("{Query: %s, PeriodSec: %d}", cq.Query, cq.PeriodSec)
}
