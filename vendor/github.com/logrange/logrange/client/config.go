// Copyright 2018-2019 The logrange Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/scanner"
	"github.com/logrange/logrange/pkg/storage"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/logrange/range/pkg/transport"
	"io/ioutil"
)

// Config struct just aggregate different types of configs in one place
type (
	Config struct {
		Forwarder *forwarder.Config
		Collector *scanner.Config
		Transport *transport.Config
		Storage   *storage.Config
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
	if cfg.Forwarder != nil {
		cfg.Forwarder.ReloadFn = func() (*forwarder.Config, error) {
			cfg, err := LoadCfgFromFile(path)
			if err != nil {
				return nil, err
			}
			return cfg.Forwarder, err
		}
	}
	return cfg, nil
}

//===================== config =====================

func NewDefaultConfig() *Config {
	return &Config{
		Forwarder: forwarder.NewDefaultConfig(),
		Collector: scanner.NewDefaultConfig(),
		Storage:   storage.NewDefaultConfig(),
		Transport: &transport.Config{
			ListenAddr: "127.0.0.1:9966",
		},
	}
}

func (c *Config) Apply(other *Config) {
	if other == nil {
		return
	}

	if c.Forwarder != nil {
		c.Forwarder.Apply(other.Forwarder)
	}
	if c.Collector != nil {
		c.Collector.Apply(other.Collector)
	}
	if c.Storage != nil {
		c.Storage.Apply(other.Storage)
	}
	if c.Transport != nil {
		c.Transport.Apply(other.Transport)
	}
}

func (c *Config) String() string {
	return utils.ToJsonStr(c)
}
