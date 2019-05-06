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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jrivets/log4g"
	"github.com/julienschmidt/httprouter"
	"github.com/logrange/logrange/api"
	"github.com/logrange/logrange/pkg/forwarder"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/mohae/deepcopy"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	Adapter struct {
		cfg   *Config
		wTmpl *forwarder.WorkerConfig

		lCli api.Client
		kCli *kCli

		waitWg sync.WaitGroup
		logger log4g.Logger
	}

	handlerWithCtx func(ctx context.Context, w http.ResponseWriter,
		r *http.Request, p httprouter.Params) error

	grLogEntry struct {
		Type    string `json:"type"`
		Payload string `json:"payload"`
	}
)

const (
	lgLimitLines     = 1000
	dlLimitLines     = 50000000
	dlLimitPerFileBt = 10 * 1024 * 1024
	dlFilename       = "messages"
)

func Run(ctx context.Context, cfg *Config, cl api.Client) error {
	logger := log4g.GetLogger("adapter")

	adaptr, err := NewAdapter(cfg, cl)
	if err != nil {
		return fmt.Errorf("failed to create adapter, err=%v", err)
	}
	if err := adaptr.Run(ctx); err != nil {
		return fmt.Errorf("failed to run adapter, err=%v", err)
	}

	<-ctx.Done()
	_ = adaptr.Close()

	logger.Info("Shutdown.")
	return err
}

//===================== adapter =====================

func NewAdapter(cfg *Config, cli api.Client) (*Adapter, error) {
	f := new(Adapter)
	f.cfg = deepcopy.Copy(cfg).(*Config)

	f.lCli = cli
	f.logger = log4g.GetLogger("adapter")
	return f, nil
}

func (ad *Adapter) Run(ctx context.Context) error {
	ad.logger.Info("Running, config=", ad.cfg)
	if err := ad.init(); err != nil {
		return err
	}

	ad.runServeAPI(ctx)
	ad.runSync(ctx)
	ad.runCronQueries(ctx)
	return nil
}

func (ad *Adapter) init() error {
	var err error
	ad.kCli, err = newKCli(ad.cfg)
	if err != nil {
		return err
	}
	ad.wTmpl, err = ad.cfg.Logrange.GetForwarderTmpl()
	if err != nil {
		return err
	}
	return err
}

func (ad *Adapter) Close() error {
	var err error
	if !utils.WaitWaitGroup(&ad.waitWg, time.Minute) {
		err = errors.New("close timeout")
	}
	ad.logger.Info("Closed, err=", err)
	return nil
}

//===================== adapter.api =====================

func (ad *Adapter) runServeAPI(ctx context.Context) {
	ad.logger.Info("Running serve API on ", ad.cfg.Gravity.ApiListenAddr)
	ad.waitWg.Add(1)
	go func() {
		srv := ad.serveAPI(ctx)
		select {
		case <-ctx.Done():
			sctx, _ := context.WithTimeout(context.Background(),
				time.Second*time.Duration(5))
			_ = srv.Shutdown(sctx)
		}
		ad.logger.Warn("Serving API stopped.")
		ad.waitWg.Done()
	}()
}

func (ad *Adapter) serveAPI(ctx context.Context) *http.Server {
	router := httprouter.New()
	router.GET("/v1/log", ad.makeHandlerWithCtx(ctx, ad.logHandler))
	router.GET("/v1/download", ad.makeHandlerWithCtx(ctx, ad.downloadHandler))

	srv := &http.Server{Addr: ad.cfg.Gravity.ApiListenAddr, Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			ad.logger.Fatal("serveAPI(): %s", err)
		}
	}()
	return srv
}

func (ad *Adapter) logHandler(ctx context.Context, rw http.ResponseWriter, rq *http.Request, p httprouter.Params) error {
	ad.logger.Info("log(): Request=", rq)
	qQuery := strings.TrimSpace(rq.URL.Query().Get("query"))
	qLimit := strings.TrimSpace(rq.URL.Query().Get("limit"))

	var (
		err error
	)

	limit := lgLimitLines
	if qLimit != "" {
		limit, err = strconv.Atoi(qLimit)
		if err != nil || limit < 0 {
			limit, err = lgLimitLines, nil
		}
	}

	qr := &api.QueryRequest{
		Query: buildLql(qQuery, ad.cfg.Logrange.Partition, limit, -limit),
		Pos:   "tail", Offset: -limit, Limit: limit,
	}

	ad.logger.Info("log(): Query=", qr.Query)
	rs := &api.QueryResult{}
	err = ad.lCli.Query(ctx, qr, rs)
	if err == nil {
		grLogEntries, err := ad.toGrLogEntries(rs.Events)
		if err == nil {
			bb, err := json.Marshal(grLogEntries)
			if err == nil {
				_, err = rw.Write(bb)
			}
		}
	}

	if err != nil {
		ad.logger.Error("log(): Err=", err)
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte("oops, something went wrong!\n"))
	}

	return err
}

func (ad *Adapter) toGrLogEntries(evs []*api.LogEvent) ([]string, error) {
	gle := make([]string, 0, len(evs))
	le := &grLogEntry{Type: "data"}
	for _, e := range evs {
		le.Payload = e.Message
		bb, err := json.Marshal(le)
		if err != nil {
			return nil, err
		}
		gle = append(gle, string(bb))
	}
	return gle, nil
}

func (ad *Adapter) downloadHandler(ctx context.Context,
	rw http.ResponseWriter, rq *http.Request, p httprouter.Params) error {

	gzWriter := gzip.NewWriter(rw)
	defer gzWriter.Close()
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	lql := buildLql("", ad.cfg.Logrange.Partition, dlLimitLines, -dlLimitLines)
	ad.logger.Info("download(): Query=", lql)

	var (
		fnCntr = 0
		err    error
	)

	buf := bytes.Buffer{}
	err = api.Select(ctx, ad.lCli, &api.QueryRequest{Query: lql,
		Limit: dlLimitLines}, false, func(res *api.QueryResult) {
		if err == nil {
			marshal(res.Events, &buf)
			if buf.Len() > dlLimitPerFileBt {
				err = writeTar(fnCntr, buf.Bytes(), tarWriter)
				buf.Reset()
				fnCntr++
			}
		}
	})

	if err == nil {
		if buf.Len() > 0 {
			err = writeTar(fnCntr, buf.Bytes(), tarWriter)
		}
	}

	if err != nil {
		ad.logger.Error("download(): Err=", err)
		_ = writeTar(fnCntr, []byte("oops, something went wrong!\n"), tarWriter)
	}

	rw.Header().Set("Content-Disposition", "attachment; filename=logs.tar.gz")
	return nil
}

func (ad *Adapter) makeHandlerWithCtx(ctx context.Context, handler handlerWithCtx) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		_ = handler(ctx, w, r, p)
	}
}

//===================== adapter.cron =====================

func (ad *Adapter) runCronQueries(ctx context.Context) {
	if ad.cfg == nil || ad.cfg.Logrange == nil {
		ad.logger.Warn("No config or Logrange config, skipping running cron queries")
		return
	}

	ad.logger.Info("Running ", len(ad.cfg.Logrange.CronQueries), " cron queries")
	for _, cq := range ad.cfg.Logrange.CronQueries {
		ad.waitWg.Add(1)
		go func(cq cronQuery) {
			defer ad.waitWg.Done()
			ad.runCronQuery(ctx, cq)
		}(cq)
	}
}

func (ad *Adapter) runCronQuery(ctx context.Context, cq cronQuery) {
	ad.logger.Info("Entering into the loop to run \"", cq.Query, "\" every ", cq.PeriodSec, " seconds.")
	for {
		sleepDelay := time.Second * time.Duration(cq.PeriodSec)
		res, err := ad.lCli.Execute(ctx, api.ExecRequest{Query: cq.Query})
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

//===================== adapter.sync =====================

func (ad *Adapter) runSync(ctx context.Context) {
	ad.logger.Info("Running sync every ", ad.cfg.SyncIntervalSec, " seconds...")
	ticker := time.NewTicker(time.Second *
		time.Duration(ad.cfg.SyncIntervalSec))

	ad.waitWg.Add(1)
	go func() {
		for utils.Wait(ctx, ticker) {
			ad.sync(ctx)
		}
		ad.logger.Warn("Sync stopped.")
		ad.waitWg.Done()
	}()
}

func (ad *Adapter) sync(ctx context.Context) {
	ad.logger.Debug("sync(): Getting Logrange forwarder config...")
	lrFwdCfg, err := ad.kCli.getLrFwdCfg()
	if err != nil {
		ad.logger.Error("sync(): Err=", err)
		return
	}

	ad.logger.Debug("sync(): Getting Gravity forwarder config...")
	grFwdCfgs, err := ad.kCli.getGrFwdCfg()
	if err != nil {
		ad.logger.Error("sync(): Err=", err)
		return
	}

	ad.logger.Debug("sync(): Merging forwarder configs...")
	newFwdCfg, err := ad.mergeFwdConfigs(lrFwdCfg.Forwarder, grFwdCfgs, ad.wTmpl)
	if err != nil {
		ad.logger.Error("sync(): Err=", err)
		return
	}

	ad.logger.Info("sync(): Updating Logrange forwarder config: from=", lrFwdCfg.Forwarder, " to=", newFwdCfg)
	lrFwdCfg.Forwarder = newFwdCfg
	if err = ad.kCli.updateLrFwdCfg(lrFwdCfg); err != nil {
		ad.logger.Error("sync(): Err=", err)
	}
}

func (ad *Adapter) mergeFwdConfigs(lrFwdCfg *forwarder.Config, grFwdCfgs []*grForwarderCfg,
	wCfgTmpl *forwarder.WorkerConfig) (*forwarder.Config, error) {

	lrFwdCfg = deepcopy.Copy(lrFwdCfg).(*forwarder.Config)
	lrFwdCfg.Workers = make([]*forwarder.WorkerConfig, 0, len(grFwdCfgs))

	for _, grCfg := range grFwdCfgs {
		wCfg := deepcopy.Copy(wCfgTmpl).(*forwarder.WorkerConfig)
		if grCfg.Metadata.Name == "" {
			ad.logger.Warn("merge(): 'name' is empty in Gravity cfg=", grCfg, ", skipping cfg...")
			continue
		}
		wCfg.Name = grCfg.Metadata.Name
		if grCfg.Spec.Address == "" {
			ad.logger.Warn("merge(): 'address' is empty in Gravity cfg=", grCfg, ", skipping cfg...")
			continue
		}
		wCfg.Sink.Params["RemoteAddr"] = grCfg.Spec.Address
		if grCfg.Spec.Protocol != "" {
			wCfg.Sink.Params["Protocol"] = grCfg.Spec.Protocol
		}
		lrFwdCfg.Workers = append(lrFwdCfg.Workers, wCfg)
	}

	return lrFwdCfg, nil
}

//===================== utils =====================

func marshal(evs []*api.LogEvent, buf *bytes.Buffer) {
	for _, e := range evs {
		buf.WriteString("{\"ts\":")
		buf.WriteString(utils.EscapeJsonStr(time.Unix(0, int64(e.Timestamp)).In(time.UTC).
			Format("2006-01-02T15:04:05.999999Z07:00")))

		buf.WriteString(", ")
		buf.WriteString("\"tags\":")
		buf.WriteString(utils.EscapeJsonStr(e.Tags))

		buf.WriteString(", ")
		buf.WriteString("\"fields\":")
		buf.WriteString(utils.EscapeJsonStr(e.Fields))

		buf.WriteString(", ")
		buf.WriteString("\"msg\":")
		buf.WriteString(utils.EscapeJsonStr(e.Message))
		buf.WriteString("}\n")
	}
}

func tarHeader(fnum int, size int) *tar.Header {
	name := dlFilename
	if fnum > 0 {
		name = fmt.Sprintf("%v.%v", dlFilename, fnum-1)
	}

	return &tar.Header{
		Name:     name,
		ModTime:  time.Now(),
		Mode:     0777,
		Typeflag: tar.TypeReg,
		Size:     int64(size),
	}
}

func writeTar(fnum int, buf []byte, tw *tar.Writer) error {
	th := tarHeader(fnum, len(buf))
	err := tw.WriteHeader(th)
	if err == nil {
		_, err = tw.Write(buf)
	}
	return err
}
