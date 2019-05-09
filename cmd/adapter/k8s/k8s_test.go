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
	"errors"
	log "github.com/gravitational/logrus"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/forwarder/sink"
	"reflect"
	"strings"
	"testing"
)

func TestClient_filterInvalidCfgs(t *testing.T) {
	type args struct {
		grFwdCfgs []*gravityForwarderCfg
	}
	tests := []struct {
		name string
		args args
		want []*gravityForwarderCfg
	}{
		{name: "test1",
			args: args{grFwdCfgs: []*gravityForwarderCfg{
				{Metadata: struct {
					Name string `yaml:"name"`
				}{Name: "name1"}},
				{Metadata: struct {
					Name string `yaml:"name"`
				}{Name: "name2"},
					Spec: struct {
						Address  string `yaml:"address"`
						Protocol string `yaml:"protocol,omitempty"`
					}{Address: "127.0.0.2", Protocol: ""},
				}}},
			want: []*gravityForwarderCfg{
				{Metadata: struct {
					Name string `yaml:"name"`
				}{Name: "name2"},
					Spec: struct {
						Address  string `yaml:"address"`
						Protocol string `yaml:"protocol,omitempty"`
					}{Address: "127.0.0.2", Protocol: ""},
				}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &Client{
				logger: log.WithField("test", "filterInvalidCfgs()"),
			}
			if got := cli.filterInvalidCfgs(tt.args.grFwdCfgs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.filterInvalidCfgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_mergeFwdConfigs(t *testing.T) {
	type args struct {
		lrFwdCfg  *forwarder.Config
		grFwdCfgs []*gravityForwarderCfg
		wCfgTmpl  *forwarder.WorkerConfig
	}
	tests := []struct {
		name string
		args args
		want *forwarder.Config
	}{
		{name: "test1",
			args: args{
				//wCfgTmpl
				wCfgTmpl: &forwarder.WorkerConfig{
					Name: "default",
					Pipe: &forwarder.PipeConfig{Name: "pipe"},
					Sink: &sink.Config{Type: "syslog", Params: map[string]interface{}{
						"Protocol": "tcp",
					}},
				},
				//grFwdCfgs
				grFwdCfgs: []*gravityForwarderCfg{
					//0
					{Metadata: struct {
						Name string `yaml:"name"`
					}{Name: "name1"},
						Spec: struct {
							Address  string `yaml:"address"`
							Protocol string `yaml:"protocol,omitempty"`
						}{Address: "127.0.0.1", Protocol: "udp"}},
					//1
					{Metadata: struct {
						Name string `yaml:"name"`
					}{Name: "name2"},
						Spec: struct {
							Address  string `yaml:"address"`
							Protocol string `yaml:"protocol,omitempty"`
						}{Address: "127.0.0.2", Protocol: ""},
					}},
				//lrFwdCfg
				lrFwdCfg: &forwarder.Config{},
			},
			want: &forwarder.Config{
				Workers: []*forwarder.WorkerConfig{{
					Name: "name1",
					Pipe: &forwarder.PipeConfig{Name: "pipe"},
					Sink: &sink.Config{
						Type: "syslog",
						Params: map[string]interface{}{
							"Protocol":   "udp",
							"RemoteAddr": "127.0.0.1",
						},
					},
				}, {
					Name: "name2",
					Pipe: &forwarder.PipeConfig{Name: "pipe"},
					Sink: &sink.Config{
						Type: "syslog",
						Params: map[string]interface{}{
							"Protocol":   "tcp",
							"RemoteAddr": "127.0.0.2",
						},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &Client{}
			got, _ := cli.mergeFwdConfigs(tt.args.lrFwdCfg, tt.args.grFwdCfgs, tt.args.wCfgTmpl)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.mergeFwdConfigs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Merge(t *testing.T) {
	type fields struct {
		Namespace              string
		ForwarderConfigMapName string
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
			fields: fields{Namespace: "ns", ForwarderConfigMapName: "cmName"},
			args:   args{other: &Config{Namespace: "ns1"}},
			want:   &Config{Namespace: "ns1", ForwarderConfigMapName: "cmName"},
		},
		{name: "test2",
			fields: fields{Namespace: "ns", ForwarderConfigMapName: "cmName"},
			args:   args{other: &Config{Namespace: "ns1", ForwarderConfigMapName: "cmName1"}},
			want:   &Config{Namespace: "ns1", ForwarderConfigMapName: "cmName1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Namespace:              tt.fields.Namespace,
				ForwarderConfigMapName: tt.fields.ForwarderConfigMapName,
			}
			cfg.Merge(tt.args.other)
			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Config.Merge() = %v, want %v", cfg, tt.want)
			}
		})
	}
}

func TestConfig_Check(t *testing.T) {
	type fields struct {
		Namespace              string
		ForwarderConfigMapName string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{name: "test1",
			fields:  fields{Namespace: "", ForwarderConfigMapName: "cmName"},
			wantErr: errors.New("invalid Namespace"),
		},
		{name: "test2",
			fields:  fields{Namespace: "ns", ForwarderConfigMapName: ""},
			wantErr: errors.New("invalid ForwarderConfigMapName"),
		},
		{name: "test3",
			fields:  fields{Namespace: "ns", ForwarderConfigMapName: "cmName"},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Namespace:              tt.fields.Namespace,
				ForwarderConfigMapName: tt.fields.ForwarderConfigMapName,
			}
			if err := cfg.Check(); (err == nil && tt.wantErr != nil) ||
				(err != nil && !strings.Contains(err.Error(), tt.wantErr.Error())) {
				t.Errorf("Config.Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
