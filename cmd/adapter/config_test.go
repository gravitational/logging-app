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
	t.Run("test1", func(t *testing.T) {
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
	})
}

func TestConfig_Merge(t *testing.T) {
	type fields struct {
		Gravity         *gravity
		Logrange        *logrange
		SyncIntervalSec int
	}
	type args struct {
		other *Config
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Config
	}{
		{name: "test1",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 123,
			},
			args: args{
				other: &Config{
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
				},
			},
			want: &Config{
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
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Gravity:         tt.fields.Gravity,
				Logrange:        tt.fields.Logrange,
				SyncIntervalSec: tt.fields.SyncIntervalSec,
			}
			c.Merge(tt.args.other)
			if !reflect.DeepEqual(c, tt.want) {
				t.Errorf("Config.Merge() = %v, want %v", c, tt.want)
			}
		})
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
		{name: "test1",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 123,
			},
			wantErr: nil,
		},

		{name: "test2",
			fields: fields{
				Gravity:         nil,
				Logrange:        newDefaultLograngeConfig(),
				SyncIntervalSec: 123,
			},
			wantErr: errors.New("invalid Gravity"),
		},

		{name: "test3",
			fields: fields{
				Gravity:         newDefaultGravityConfig(),
				Logrange:        nil,
				SyncIntervalSec: 123,
			},
			wantErr: errors.New("invalid Logrange"),
		},

		{name: "test4",
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
	type fields struct {
		ApiListenAddr string
		Kubernetes    *k8s.Config
	}
	type args struct {
		other *gravity
	}

	defaultGravityCfg := newDefaultGravityConfig()
	mergeGravityCfg := &gravity{
		ApiListenAddr: "127.0.0.123:1234",
		Kubernetes:    &k8s.Config{Namespace: "gravity", ForwarderConfigMapName: "cmName"},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *gravity
	}{
		{name: "test1",
			fields: fields{
				ApiListenAddr: defaultGravityCfg.ApiListenAddr,
				Kubernetes:    defaultGravityCfg.Kubernetes,
			},
			args: args{other: mergeGravityCfg},
			want: mergeGravityCfg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &gravity{
				ApiListenAddr: tt.fields.ApiListenAddr,
				Kubernetes:    tt.fields.Kubernetes,
			}
			g.merge(tt.args.other)
			if !reflect.DeepEqual(g, tt.want) {
				t.Errorf("gravity.merge() = %v, want %v", g, tt.want)
			}
		})
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
		{name: "test1",
			fields: fields{
				ApiListenAddr: defaultGravityCfg.ApiListenAddr,
				Kubernetes:    defaultGravityCfg.Kubernetes,
			},
			wantErr: nil,
		},

		{name: "test2",
			fields: fields{
				ApiListenAddr: "",
				Kubernetes:    defaultGravityCfg.Kubernetes,
			},
			wantErr: errors.New("invalid ApiListenAddr"),
		},

		{name: "test3",
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
	type fields struct {
		CronQueries       []cronQuery
		Partition         string
		ForwarderTmplFile string
		Kubernetes        *k8s.Config
		Transport         *transport.Config
	}
	type args struct {
		other *logrange
	}

	defaultLograngeCfg := newDefaultLograngeConfig()
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

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *logrange
	}{
		{name: "test1",
			fields: fields{
				CronQueries:       defaultLograngeCfg.CronQueries,
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			args: args{
				other: mergeLograngeCfg,
			},
			want: mergeLograngeCfg,
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

			l.merge(tt.args.other)
			if !reflect.DeepEqual(l, tt.want) {
				t.Errorf("logrange.merge() = %v, want %v", l, tt.want)
			}
		})
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
		{name: "test1",
			fields: fields{
				CronQueries:       defaultLograngeCfg.CronQueries,
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: nil,
		},

		{name: "test2",
			fields: fields{
				Partition:         "",
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid Partition"),
		},

		{name: "test3",
			fields: fields{
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: "",
				Kubernetes:        defaultLograngeCfg.Kubernetes,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid ForwarderTmplFile"),
		},

		{name: "test4",
			fields: fields{
				Partition:         defaultLograngeCfg.Partition,
				ForwarderTmplFile: defaultLograngeCfg.ForwarderTmplFile,
				Kubernetes:        nil,
				Transport:         defaultLograngeCfg.Transport,
			},
			wantErr: errors.New("invalid Kubernetes"),
		},

		{name: "test5",
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
