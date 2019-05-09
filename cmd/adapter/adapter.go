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
	"github.com/gravitational/logging-app/cmd/adapter/api"
	"github.com/gravitational/logging-app/cmd/adapter/k8s"
	log "github.com/gravitational/logrus"
	"github.com/gravitational/trace"
	lapi "github.com/logrange/logrange/api"
	"github.com/logrange/logrange/pkg/utils"
	"sync"
	"time"
)

type (
	// Gravity has certain expectations regarding the interface
	// its logging application exposes to an end user. Current
	// logging application implementation is based on Logrange database
	// which has it's own user interface and which is pretty different from
	// what is expected by Gravity. The Adapter is intended to be
	// an actual adapter which sits on top of Logrange interface
	// and exposes Gravity expected interface.
	//
	// Adapter takes the responsibility of making all the needed transformations and
	// configuration synchronizations in between Gravity and Logrange in order to
	// meet Gravity logging application requirements.
	//
	// Adapter has certain lifecycle and it's
	// caller responsibility to call Start() and Close() appropriately.
	//
	Adapter struct {
		cfg Config

		// Logrange client
		lrClient lapi.Client
		// K8s client
		k8sClient *k8s.Client
		// Wait group to wait async jobs (started goroutines)
		wg sync.WaitGroup

		logger *log.Entry
	}
)

// Runs new Adapter instance and waits till its execution ends,
// context is cancelled or error happens.
func Run(ctx context.Context, cfg Config, cl lapi.Client) error {
	adaptr, err := NewAdapter(cfg, cl)
	if err != nil {
		return trace.WrapWithMessage(err, "failed to create adapter")
	}
	if err := adaptr.Start(ctx); err != nil {
		return trace.WrapWithMessage(err, "failed to start adapter")
	}

	<-ctx.Done()
	_ = adaptr.Close()

	adaptr.logger.Info("Shutdown.")
	return nil
}

// Creates new Adapter instance. Adapter has certain lifecycle and it's
// caller responsibility to call Start() and Close() appropriately.
func NewAdapter(cfg Config, cli lapi.Client) (*Adapter, error) {
	f := new(Adapter)
	f.cfg = cfg
	f.lrClient = cli
	f.logger = log.WithField(trace.Component, "logging-app.adapter")
	return f, nil
}

// Starts adapter which includes starting goroutines for serving API requests
// and running recurring jobs (sync, cronQueries), the method is non-blocking
// and passed context controls adapter's lifespan (including started goroutines)
func (ad *Adapter) Start(ctx context.Context) error {
	ad.logger.Info("Starting, config=", ad.cfg)
	if err := ad.init(); err != nil {
		return trace.Wrap(err)
	}

	ad.runApiServer(ctx)
	ad.runSync(ctx)
	ad.runCronQueries(ctx)
	return nil
}

func (ad *Adapter) init() error {
	wTmpl, err := ad.cfg.Logrange.getForwarderTmpl()
	if err != nil {
		return trace.Wrap(err)
	}
	ad.k8sClient, err = k8s.NewClient(ad.cfg.Gravity.Kubernetes,
		ad.cfg.Logrange.Kubernetes, wTmpl)
	return trace.Wrap(err)
}

// The method ensures that Adapter shutdown has finished
// and all the related jobs (goroutines) are stopped.
// The method has timeout (1 min by default), if shutdown
// didn't finish during that time the method returns error.
func (ad *Adapter) Close() error {
	var err error
	if !utils.WaitWaitGroup(&ad.wg, time.Minute) {
		err = trace.Errorf("close timeout")
	}
	ad.logger.Info("Closed, err=", err)
	return trace.Wrap(err)
}

func (ad *Adapter) runApiServer(ctx context.Context) {
	ad.logger.Info("Running serve API on ", ad.cfg.Gravity.ApiListenAddr)
	ad.wg.Add(1)
	go func() {
		srv := api.NewServer(ad.cfg.Gravity.ApiListenAddr, ad.lrClient, ad.cfg.Logrange.Partition)
		go func() {
			srv.Serve(ctx)
		}()
		select {
		case <-ctx.Done():
			_ = srv.Shutdown()
		}
		ad.logger.Warn("Serving API stopped.")
		ad.wg.Done()
	}()
}

func (ad *Adapter) runCronQueries(ctx context.Context) {
	ad.logger.Info("Running ", len(ad.cfg.Logrange.CronQueries), " cron queries: ", ad.cfg.Logrange.CronQueries)
	for _, cq := range ad.cfg.Logrange.CronQueries {
		ad.wg.Add(1)
		go func(cq cronQuery) {
			defer ad.wg.Done()
			ad.runCronQuery(ctx, cq)
		}(cq)
	}
}

func (ad *Adapter) runCronQuery(ctx context.Context, cq cronQuery) {
	ad.logger.Infof("Entering into the loop to run %q every %v seconds.", cq.Query, cq.PeriodSec)
	for {
		sleepDelay := time.Second * time.Duration(cq.PeriodSec)
		res, err := ad.lrClient.Execute(ctx, lapi.ExecRequest{Query: cq.Query})
		if err != nil {
			ad.logger.Warn("Could not connect to the server to run ", cq)
			sleepDelay = time.Second
		}
		if res.Err != nil {
			ad.logger.Error("Server returned error on executing ", cq.Query, " the err=", res.Err)
		}

		select {
		case <-ctx.Done():
			ad.logger.Info("Breaking up the runCronQuery loop for ", cq, " the context is closed.")
			return
		case <-time.After(sleepDelay):
		}
	}
}

func (ad *Adapter) runSync(ctx context.Context) {
	ad.logger.Info("Running sync every ", ad.cfg.SyncIntervalSec, " seconds...")
	ticker := time.NewTicker(time.Second *
		time.Duration(ad.cfg.SyncIntervalSec))

	ad.wg.Add(1)
	go func() {
		for utils.Wait(ctx, ticker) {
			ad.k8sClient.SyncForwarders(ctx)
		}
		ad.logger.Warn("Sync stopped.")
		ad.wg.Done()
	}()
}
