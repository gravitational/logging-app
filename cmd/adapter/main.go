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
	"fmt"
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
	argCfgFile    = "config-file"
	argLogCfgFile = "log-config-file"

	argServerAddr    = "server-addr"
	argAPIListenAddr = "api-listen-addr"
)

var (
	cfg    = NewDefaultConfig()
	logger = log4g.GetLogger("adapter")
)

func main() {
	defer log4g.Shutdown()

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
					&ucli.StringFlag{
						Name:  argLogCfgFile,
						Usage: "log4g configuration file path",
					},
				},
			},
		},
	}

	sort.Sort(ucli.FlagsByName(app.Flags))
	sort.Sort(ucli.FlagsByName(app.Commands[0].Flags))
	if err := app.Run(os.Args); err != nil {
		logger.Error(err)
	}
}

func initCfg(c *ucli.Context) error {
	var (
		err error
	)

	logCfgFile := c.String(argLogCfgFile)
	if logCfgFile != "" {
		err = log4g.ConfigF(logCfgFile)
		if err != nil {
			return err
		}
	}

	cfgFile := c.String(argCfgFile)
	if cfgFile != "" {
		logger.Info("Loading config from=", cfgFile)
		config, err := LoadCfgFromFile(cfgFile)
		if err != nil {
			return err
		}
		cfg.Apply(config)
	}

	applyArgsToCfg(c, cfg)
	return nil
}

func applyArgsToCfg(c *ucli.Context, cfg *Config) {
	if sa := c.String(argServerAddr); sa != "" {
		cfg.Logrange.Transport.ListenAddr = sa
	}
	if aa := c.String(argAPIListenAddr); aa != "" {
		cfg.Gravity.ApiListenAddr = aa
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

//===================== adapter =====================

func runAdapter(c *ucli.Context) error {
	err := initCfg(c)
	if err != nil {
		return err
	}

	cli, err := rpc.NewClient(*cfg.Logrange.Transport)
	if err != nil {
		return fmt.Errorf("failed to create client, err=%v", err)
	}

	defer cli.Close()
	return Run(newCtx(), cfg, cli)
}
