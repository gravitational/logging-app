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
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/logrange/range/pkg/transport"
	"io/ioutil"
)

type (
	gravity struct {
		ApiListenAddr string
		Kubernetes    *kube
	}

	kube struct {
		Namespace              string
		ForwarderConfigMapName string
	}

	// cronQuery describe a query, which should be called periodically
	cronQuery struct {
		// Query contains LQL which should be called periodically
		Query string
		// PeriodSec defines an interval in seconds to execute the Query
		PeriodSec int
	}

	logrange struct {
		CronQueries       []cronQuery
		Partition         string
		ForwarderTmplFile string
		Kubernetes        *kube
		Transport         *transport.Config
	}

	Config struct {
		Gravity         *gravity
		Logrange        *logrange
		SyncIntervalSec int
	}
)

func LoadCfgFromFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

//===================== config =====================

func NewDefaultConfig() *Config {
	return &Config{
		Gravity: &gravity{
			ApiListenAddr: "127.0.0.1:8083",
			Kubernetes: &kube{
				Namespace:              "kube-system",
				ForwarderConfigMapName: "log-forwarders",
			},
		},

		Logrange: &logrange{
			Partition:         "logrange.pipe=__default__",
			ForwarderTmplFile: "/opt/logrange/gravity/config/forward-tmpl.json",
			Kubernetes: &kube{
				Namespace:              "kube-system",
				ForwarderConfigMapName: "lr-forwarder",
			},
			Transport: &transport.Config{
				ListenAddr: "127.0.0.1:9966",
			},
		},

		SyncIntervalSec: 5,
	}
}

func (c *Config) Apply(other *Config) {
	if other == nil {
		return
	}

	if other.Gravity != nil {
		c.Gravity.apply(other.Gravity)
	}
	if other.Logrange != nil {
		c.Logrange.apply(other.Logrange)
	}
	if other.SyncIntervalSec != 0 {
		c.SyncIntervalSec = other.SyncIntervalSec
	}
}

func (c *Config) check() error {
	if c.Gravity == nil {
		return fmt.Errorf("invalid Gravity=%v, must be non-nil", c.Gravity)
	}
	if c.Logrange == nil {
		return fmt.Errorf("invalid Logrange=%v, must be non-nil", c.Logrange)
	}
	if err := c.Gravity.check(); err != nil {
		return fmt.Errorf("invalid Gravity=%v: %v", c.Gravity, err)
	}
	if err := c.Logrange.check(); err != nil {
		return fmt.Errorf("invalid Logrange=%v: %v", c.Logrange, err)
	}
	if c.SyncIntervalSec <= 0 {
		return fmt.Errorf("invalid SyncIntervalSec=%v, must be > 0sec", c.SyncIntervalSec)
	}
	return nil
}

func (c *Config) String() string {
	return utils.ToJsonStr(c)
}

//===================== gravity =====================

func (g *gravity) apply(other *gravity) {
	if other == nil {
		return
	}

	if other.Kubernetes != nil {
		g.Kubernetes.apply(other.Kubernetes)
	}
	if other.ApiListenAddr != "" {
		g.ApiListenAddr = other.ApiListenAddr
	}
}

func (g *gravity) check() error {
	if g.Kubernetes.check() == nil {
		return fmt.Errorf("invalid Kubernetes=%v: must be non-nil", g.Kubernetes)
	}
	if err := g.Kubernetes.check(); err != nil {
		return fmt.Errorf("invalid Kubernetes=%v: %v", g.Kubernetes, err)
	}
	if g.ApiListenAddr == "" {
		return fmt.Errorf("invalid ApiListenAddr=%v, must be non-empty", g.ApiListenAddr)
	}
	return nil
}

func (g *gravity) String() string {
	return utils.ToJsonStr(g)
}

//===================== logrange =====================

func (l *logrange) apply(other *logrange) {
	if other == nil {
		return
	}

	if other.Transport != nil {
		l.Transport.Apply(other.Transport)
	}
	if other.Kubernetes != nil {
		l.Kubernetes.apply(other.Kubernetes)
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
	if l.Kubernetes.check() == nil {
		return fmt.Errorf("invalid Kubernetes=%v: must be non-nil", l.Kubernetes)
	}
	if err := l.Kubernetes.check(); err != nil {
		return fmt.Errorf("invalid Kubernetes=%v: %v", l.Kubernetes, err)
	}
	if l.Transport == nil {
		return fmt.Errorf("invalid Transport=%v: must be non-nil", l.Transport)
	}
	if err := l.Transport.Check(); err != nil {
		return fmt.Errorf("invalid Transport=%v: %v", l.Transport, err)
	}
	if l.ForwarderTmplFile == "" {
		return fmt.Errorf("invalid ForwarderTmplFile=%v: must be non-empty", l.ForwarderTmplFile)
	}
	if _, err := l.GetForwarderTmpl(); err != nil {
		return fmt.Errorf("invalid ForwarderTmplFile=%v: %v", l.ForwarderTmplFile, err)
	}
	if l.Partition == "" {
		return fmt.Errorf("invalid Partition=%v: must be non-empty", l.Partition)
	}
	return nil
}

func (l *logrange) GetForwarderTmpl() (*forwarder.WorkerConfig, error) {
	buf, err := ioutil.ReadFile(l.ForwarderTmplFile)
	if err != nil {
		return nil, err
	}
	wCfgTmpl := &forwarder.WorkerConfig{}
	if err = json.Unmarshal(buf, wCfgTmpl); err != nil {
		return nil, err
	}
	return wCfgTmpl, nil
}

func (l *logrange) String() string {
	return utils.ToJsonStr(l)
}

//===================== kube =====================

func (k *kube) apply(other *kube) {
	if other == nil {
		return
	}

	if other.Namespace != "" {
		k.Namespace = other.Namespace
	}
	if other.ForwarderConfigMapName != "" {
		k.ForwarderConfigMapName = other.ForwarderConfigMapName
	}
}

func (k *kube) check() error {
	if k.Namespace == "" {
		return fmt.Errorf("invalid Namespace=%v: must be non-empty", k.Namespace)
	}
	if k.ForwarderConfigMapName == "" {
		return fmt.Errorf("invalid ForwarderConfigMapName=%v: must be non-empty", k.ForwarderConfigMapName)
	}
	return nil
}

func (k *kube) String() string {
	return utils.ToJsonStr(k)
}

//===================== cronQuery =====================

func (cq cronQuery) String() string {
	return fmt.Sprintf("{Query: %s, PeriodSec: %d}", cq.Query, cq.PeriodSec)
}
