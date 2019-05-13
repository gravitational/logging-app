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
	"errors"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/utils"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gravitational/logging-app/cmd/adapter/k8s"
	"github.com/logrange/range/pkg/transport"
)

func TestLoadCfgFromFile(t *testing.T) {
	// write
	cfg := NewDefaultConfig()
	bb, _ := json.Marshal(cfg)
	fd, _ := writeTmpJsonFile(bb)
	defer os.Remove(fd.Name())

	// read
	got, err := LoadCfgFromFile(fd.Name())
	if err != nil {
		t.Errorf("LoadCfgFromFile() error = %v", err)
		return
	}

	// check
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("LoadCfgFromFile() = %v, want %v", got, cfg)
	}
}

func TestConfig_Merge(t *testing.T) {
	other := &Config{
		Gravity: &gravity{
			ApiListenAddr: "127.0.0.123:1234",
			Kubernetes:    &k8s.Config{Namespace: "gravity"},
		},
		Logrange: &logrange{
			Partition:         "partition",
			Kubernetes:        &k8s.Config{Namespace: "logrange"},
			ForwarderTmplFile: "file1",
			Transport:         &transport.Config{Tls2Way: utils.BoolPtr(true)},
		},
	}

	want := &Config{
		Gravity: &gravity{
			ApiListenAddr: "127.0.0.123:1234",
			Kubernetes: &k8s.Config{
				Namespace:              "gravity",
				ForwarderConfigMapName: "log-forwarders",
			},
		},
		Logrange: &logrange{
			Partition:         "partition",
			ForwarderTmplFile: "file1",
			Kubernetes: &k8s.Config{
				Namespace:              "logrange",
				ForwarderConfigMapName: "lr-forwarder",
			},
			Transport: &transport.Config{
				ListenAddr: "127.0.0.1:9966",
				Tls2Way:    utils.BoolPtr(true),
			},
		},
		SyncIntervalSec: 123,
	}

	c := &Config{
		Gravity:         newDefaultGravityConfig(),
		Logrange:        newDefaultLograngeConfig(),
		SyncIntervalSec: 123,
	}

	c.Merge(other)
	if !reflect.DeepEqual(c, want) {
		t.Errorf("Config.Merge() = %v, want %v", c, want)
	}
}

func TestConfig_Check(t *testing.T) {
	type fields struct {
		Gravity         *gravity
		Logrange        *logrange
		SyncIntervalSec int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{name: "check config ok",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 123,
			},
			wantErr: nil,
		},

		{name: "check invalid Gravity err",
			fields: fields{
				Gravity:         nil,
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 123,
			},
			wantErr: errors.New("invalid Gravity"),
		},

		{name: "check invalid Logrange err",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        nil,
				SyncIntervalSec: 123,
			},
			wantErr: errors.New("invalid Logrange"),
		},

		{name: "check invalid SyncIntervalSec err",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 0,
			},
			wantErr: errors.New("invalid SyncIntervalSec"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Gravity:         tt.fields.Gravity,
				Logrange:        tt.fields.Logrange,
				SyncIntervalSec: tt.fields.SyncIntervalSec,
			}

			// substitute ForwarderTmplFile since file existence is checked during Check()
			if c.Logrange != nil && c.Logrange.ForwarderTmplFile != "" {
				cfg := forwarder.NewDefaultConfig()
				bb, _ := json.Marshal(cfg)
				fd, _ := writeTmpJsonFile(bb)
				defer os.Remove(fd.Name())
				c.Logrange.ForwarderTmplFile = fd.Name()
			}

			if err := c.Check(); (err == nil && tt.wantErr != nil) ||
				(err != nil && !strings.Contains(err.Error(), tt.wantErr.Error())) {
				t.Errorf("Config.Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_gravity_merge(t *testing.T) {
	defaultGravityCfg := newDefaultGravityConfig()
	mergeGravityCfg := &gravity{
		ApiListenAddr: "127.0.0.123:1234",
		Kubernetes:    &k8s.Config{Namespace: "gravity", ForwarderConfigMapName: "cmName"},
	}

	g := &gravity{
		ApiListenAddr: defaultGravityCfg.ApiListenAddr,
		Kubernetes:    defaultGravityCfg.Kubernetes,
	}
	g.merge(mergeGravityCfg)
	if !reflect.DeepEqual(g, mergeGravityCfg) {
		t.Errorf("gravity.merge() = %v, want %v", g, mergeGravityCfg)
	}
}

func Test_gravity_check(t *testing.T) {
	type fields struct {
		ApiListenAddr string
		Kubernetes    *k8s.Config
	}

	defaultGravityCfg := newDefaultGravityConfig()
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{name: "check config ok",
			fields: fields{
				ApiListenAddr: defaultGravityCfg.ApiListenAddr,
				Kubernetes:    defaultGravityCfg.Kubernetes,
			},
			wantErr: nil,
		},

		{name: "check invalid ApiListenAddr err",
			fields: fields{
				ApiListenAddr: "",
				Kubernetes:    defaultGravityCfg.Kubernetes,
			},
			wantErr: errors.New("invalid ApiListenAddr"),
		},

		{name: "check invalid Kubernetes err",
			fields: fields{
				ApiListenAddr: defaultGravityCfg.ApiListenAddr,
				Kubernetes:    nil,
			},
			wantErr: errors.New("invalid Kubernetes"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &gravity{
				ApiListenAddr: tt.fields.ApiListenAddr,
				Kubernetes:    tt.fields.Kubernetes,
			}
			if err := g.check(); (err == nil && tt.wantErr != nil) ||
				(err != nil && !strings.Contains(err.Error(), tt.wantErr.Error())) {
				t.Errorf("gravity.check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_logrange_merge(t *testing.T) {
	mergeLograngeCfg := &logrange{
		Partition:         "partition",
		ForwarderTmplFile: "file1",
		CronQueries: []cronQuery{
			{Query: "query", PeriodSec: 123},
		},
		Kubernetes: &k8s.Config{
			Namespace:              "logrange",
			ForwarderConfigMapName: "forwarder",
		},
		Transport: &transport.Config{
			ListenAddr:  "127.0.0.100:1234",
			Tls2Way:     utils.BoolPtr(true),
			TlsCertFile: "file1",
			TlsEnabled:  utils.BoolPtr(true),
		},
	}

	defaultLograngeCfg := newDefaultLograngeConfig()
	l := &logrange{
		CronQueries:       defaultLograngeCfg.CronQueries,
		Partition:         defaultLograngeCfg.Partition,
		ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
		Kubernetes:        defaultLograngeCfg.Kubernetes,
		Transport:         defaultLograngeCfg.Transport,
	}

	l.merge(mergeLograngeCfg)
	if !reflect.DeepEqual(l, mergeLograngeCfg) {
		t.Errorf("logrange.merge() = %v, want %v", l, mergeLograngeCfg)
	}
}

func Test_logrange_check(t *testing.T) {
	type fields struct {
		CronQueries       []cronQuery
		Partition         string
		ForwarderTmplFile string
		Kubernetes        *k8s.Config
		Transport         *transport.Config
	}

	defaultLograngeCfg := newDefaultLograngeConfig()
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{name: "check config ok",
			fields: fields{
				CronQueries:       defaultLograngeCfg.CronQueries,
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: nil,
		},

		{name: "check invalid Partition err",
			fields: fields{
				Partition:         "",
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid Partition"),
		},

		{name: "check invalid ForwarderTmplFile err",
			fields: fields{
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: "",
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid ForwarderTmplFile"),
		},

		{name: "check invalid Kubernetes err",
			fields: fields{
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        nil,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid Kubernetes"),
		},

		{name: "check invalid Transport err",
			fields: fields{
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         nil,
			},
			wantErr: errors.New("invalid Transport"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &logrange{
				CronQueries:       tt.fields.CronQueries,
				Partition:         tt.fields.Partition,
				ForwarderTmplFile: tt.fields.ForwarderTmplFile,
				Kubernetes:        tt.fields.Kubernetes,
				Transport:         tt.fields.Transport,
			}

			// substitute ForwarderTmplFile since file existence is checked during check()
			if l.ForwarderTmplFile != "" {
				cfg := forwarder.NewDefaultConfig()
				bb, _ := json.Marshal(cfg)
				fd, _ := writeTmpJsonFile(bb)
				defer os.Remove(fd.Name())
				l.ForwarderTmplFile = fd.Name()
			}

			if err := l.check(); (err == nil && tt.wantErr != nil) ||
				(err != nil && !strings.Contains(err.Error(), tt.wantErr.Error())) {
				t.Errorf("logrange.check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func writeTmpJsonFile(bb []byte) (*os.File, error) {
	tmpFile, _ := ioutil.TempFile(os.TempDir(), "test-")
	_, err := tmpFile.Write(bb)
	return tmpFile, err
}
