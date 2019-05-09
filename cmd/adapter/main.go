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
	"context"
	log "github.com/gravitational/logrus"
	"github.com/gravitational/trace"
	"github.com/jrivets/log4g"
	"github.com/logrange/logrange/api/rpc"
	"github.com/logrange/logrange/pkg/utils"
	ucli "gopkg.in/urfave/cli.v2"
	"os"
	"sort"
)

const (
	Version = "0.1.0"
)

const (
	// Config file path
	argCfgFile = "config-file"

	// Logrange server address (remote)
	argServerAddr = "server-addr"

	// HTTP server address to listen (local)
	argAPIListenAddr = "api-listen-addr"
)

var (
	cfg    = NewDefaultConfig()
	logger = log.WithField(trace.Component, "logging-app.main")
)

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})
	log4g.SetLogLevel("", log4g.FATAL) // mute imported log4g libs

	app := &ucli.App{
		Name:    "adapter",
		Version: Version,
		Usage:   "Gravity adapter",
		Commands: []*ucli.Command{
			{
				Name:   "start",
				Usage:  "Run gravity adapter",
				Action: runAdapter,
				Flags: []ucli.Flag{
					&ucli.StringFlag{
						Name:  argServerAddr,
						Usage: "server address",
					},
					&ucli.StringFlag{
						Name:  argAPIListenAddr,
						Usage: "api listen address",
					},
					&ucli.StringFlag{
						Name:  argCfgFile,
						Usage: "configuration file path",
					},
				},
			},
		},
	}

	sort.Sort(ucli.FlagsByName(app.Flags))
	sort.Sort(ucli.FlagsByName(app.Commands[0].Flags))
	if err := app.Run(os.Args); err != nil {
		logger.Fatal(trace.DebugReport(err))
	}
}

func initCfg(c *ucli.Context) error {
	cfgFile := c.String(argCfgFile)
	if cfgFile != "" {
		logger.Info("Loading config from=", cfgFile)
		config, err := LoadCfgFromFile(cfgFile)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Merge(config)
	}

	applyArgsToCfg(c, cfg)
	if err := cfg.Check(); err != nil {
		return trace.Wrap(err, "invalid config")
	}
	return nil
}

func applyArgsToCfg(c *ucli.Context, cfg *Config) {
	if addr := c.String(argServerAddr); addr != "" {
		cfg.Logrange.Transport.ListenAddr = addr
	}
	if addr := c.String(argAPIListenAddr); addr != "" {
		cfg.Gravity.ApiListenAddr = addr
	}
}

func newCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	utils.NewNotifierOnIntTermSignal(func(s os.Signal) {
		logger.Warn("Handling signal=", s)
		cancel()
	})
	return ctx
}

func runAdapter(c *ucli.Context) error {
	err := initCfg(c)
	if err != nil {
		return trace.Wrap(err)
	}

	cli, err := rpc.NewClient(*cfg.Logrange.Transport)
	if err != nil {
		return trace.WrapWithMessage(err, "failed to create Logrange client")
	}

	defer cli.Close()
	return Run(newCtx(), *cfg, cli)
}
