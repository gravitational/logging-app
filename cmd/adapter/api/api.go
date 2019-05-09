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

package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gravitational/logging-app/cmd/adapter/query"
	log "github.com/gravitational/logrus"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/logrange/logrange/api"
	"github.com/logrange/logrange/pkg/utils"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type (
	// Http API server intended to serve Gravity log queries,
	// internally the incoming queries are transformed to LQL (Logrange Query Language)
	// and executed against Logrange database.
	Server struct {
		// Server listen address
		listenAddr string
		// Http server
		server *http.Server
		// Logrange client query
		lrClient api.Client
		// Logrange query partition
		lrPartition string

		logger *log.Entry
	}

	// Http request handle but with context support
	handlerWithCtx func(ctx context.Context, w http.ResponseWriter,
		r *http.Request, p httprouter.Params) error

	// Http respose writer but with status exposed
	responseWriterWithStatus struct {
		http.ResponseWriter
		status int
	}

	// Represents gravitational log entry
	grLogEntry struct {
		Type    string `json:"type"`
		Payload string `json:"payload"`
	}

	// Compressed tarball writer
	tarGzEntryWriter struct {
		entryNum  int
		entryPrfx string

		gzWriter  *gzip.Writer
		tarWriter *tar.Writer
	}
)

const (
	// Log tail default limit
	defaultTailLinesLimit = 1000

	// Log download maximum number of lines
	downloadLinesMax = 50000000

	// Log download limit in bytes per file
	downloadBytesPerFileLimit = 10 * 1024 * 1024

	// Log download filename prefix
	downloadFilenamePrfx = "messages"
)

// Creates new server for the given params
func NewServer(listenAddr string, lrClient api.Client, lrPartition string) *Server {
	return &Server{
		listenAddr:  listenAddr,
		lrClient:    lrClient,
		lrPartition: lrPartition,
		logger:      log.WithField(trace.Component, "logging-app.api"),
	}
}

// Starts serving requests on the configured port, blocking, always returns a non-nil error
func (s *Server) Serve(ctx context.Context) {
	router := httprouter.New()
	router.GET("/v1/log", s.makeHandlerWithCtx(ctx, s.logHandler))
	router.GET("/v1/download", s.makeHandlerWithCtx(ctx, s.downloadHandler))

	s.server = &http.Server{Addr: s.listenAddr, Handler: router}
	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		s.logger.Fatal("serve(): ", err)
	}
}

// Gracefully shuts down the server,
// blocks till shutdown, error or timeout happens (5 sec by default)
func (s *Server) Shutdown() error {
	if s.server != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(sctx); err != nil {
			return trace.Wrap(err)
		}
	}

	s.server = nil
	return nil
}

// "/v1/log" api handler, returns logs from tail for the given params:
//
// - 'query':
// 	 	allowed query terms: "pod", "container", "file", "or", "and"
//   	example: query="pod:p1 and container:c1 and file:f1 or file:f2"
// - 'limit':
//      allowed values: int >= 0
//	 	example: limit=100
//
// In case of error it returns the error (no response write happens) so it's up to
// caller to handle it properly, e.g. return appropriate http err and code.
//
func (s *Server) logHandler(ctx context.Context, rw http.ResponseWriter, rq *http.Request, p httprouter.Params) error {
	var (
		err error
	)

	// get query params
	queryParam := strings.TrimSpace(rq.URL.Query().Get("query"))
	limitParam := strings.TrimSpace(rq.URL.Query().Get("limit"))

	// validate and set limit
	limit := defaultTailLinesLimit
	if limitParam != "" {
		limit, err = strconv.Atoi(limitParam)
		if err != nil || limit < 0 {
			s.logger.Warn("log(): Bad limit=", limitParam, ", using default; err=", err)
			limit, err = defaultTailLinesLimit, nil
		}
	}

	// build Logrange query
	qr := s.buildQueryRequest(queryParam, limit)
	s.logger.Info("log(): Query=", qr.Query)

	// execute and if executed ok, transform to Gravity format, marshal and write response
	res := &api.QueryResult{}
	err = s.lrClient.Query(ctx, qr, res)
	if err == nil {
		var logEntries []string
		logEntries, err = toGravityLogEntries(res.Events)
		if err == nil {
			var logEntriesBytes []byte
			logEntriesBytes, err = json.Marshal(logEntries)
			if err == nil {
				_, err = rw.Write(logEntriesBytes)
			}
		}
	}

	return trace.Wrap(err)
}

// "/v1/download" api handler, returns compressed tarball stream of logs from tail
//
// No query params are supported.
//
// In case of error it returns the error so it's up to caller to handle it properly,
// e.g. return appropriate http err and code.
//
// Please note, that if an error occur after a few successful response writes
// end user will get file that does not contain all the requested data.
//
func (s *Server) downloadHandler(ctx context.Context,
	rw http.ResponseWriter, rq *http.Request, p httprouter.Params) error {
	rw.Header().Set("Content-Disposition", "attachment; filename=logs.tar.gz")

	// prepare stream writer
	tgEntryWriter := newTarGzEntryWriter(rw, downloadFilenamePrfx)
	defer tgEntryWriter.close()

	// build Logrange query
	qr := s.buildQueryRequest("", downloadLinesMax)
	s.logger.Info("download(): Query=", qr.Query)

	var (
		errQ error // query error
		errW error // write error
	)

	// execute Logrange query and write tar.gz stream
	buf := bytes.Buffer{}
	errQ = api.Select(ctx, s.lrClient, qr, false,
		func(res *api.QueryResult) {
			if errW == nil {
				writeEvents(res.Events, &buf)
				if buf.Len() > downloadBytesPerFileLimit {
					errW = tgEntryWriter.write(buf.Bytes())
					buf.Reset()
				}
			}
		})

	// return query err if any
	if errQ != nil {
		return trace.Wrap(errQ)
	}
	// no write err and still some data, write it now
	if errW == nil && buf.Len() > 0 {
		errW = tgEntryWriter.write(buf.Bytes())
	}

	return trace.Wrap(errW)
}

func (s *Server) buildQueryRequest(q string, limit int) *api.QueryRequest {
	return &api.QueryRequest{
		Query: query.BuildTailLqlQuery(q, s.lrPartition, limit, -limit),
		Pos:   "tail", Offset: -limit, Limit: limit,
	}
}

// Wrapper for http handler, besides calling the actual handler it
// tries to handle returned errors (if any). In particular,
// it logs the request, error and writes StatusInternalServerError error
func (s *Server) makeHandlerWithCtx(ctx context.Context, handler handlerWithCtx) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rw := &responseWriterWithStatus{ResponseWriter: w}
		err := handler(ctx, rw, r, p)
		if err != nil {
			s.logger.Error("Request=", r, "; err=", err)
			if rw.status == 0 { // write err/status if no writes before
				trace.WriteError(rw, err)
			}
		}
	}
}

func (w *responseWriterWithStatus) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriterWithStatus) Write(b []byte) (int, error) {
	w.status = http.StatusOK
	return w.ResponseWriter.Write(b)
}

func newTarGzEntryWriter(w io.Writer, entryPrfx string) *tarGzEntryWriter {
	tgWriter := new(tarGzEntryWriter)
	tgWriter.entryPrfx = entryPrfx
	tgWriter.gzWriter = gzip.NewWriter(w)
	tgWriter.tarWriter = tar.NewWriter(tgWriter.gzWriter)
	return tgWriter
}

func (w *tarGzEntryWriter) write(b []byte) error {
	err := w.tarWriter.WriteHeader(w.nextEntryHeader(len(b)))
	if err == nil {
		_, err = w.tarWriter.Write(b)
	}
	return trace.Wrap(err)
}

func (w *tarGzEntryWriter) nextEntryHeader(size int) *tar.Header {
	name := w.entryPrfx
	if w.entryNum > 0 {
		name = fmt.Sprintf("%v.%v", w.entryPrfx, w.entryNum-1)
	}
	w.entryNum++
	return &tar.Header{
		Name:     name,
		ModTime:  time.Now(),
		Mode:     0777,
		Typeflag: tar.TypeReg,
		Size:     int64(size),
	}
}

func (w *tarGzEntryWriter) close() {
	if w.entryNum > 0 {
		_ = w.tarWriter.Close()
		_ = w.gzWriter.Close()
	}
}

func toGravityLogEntries(evs []*api.LogEvent) ([]string, error) {
	entries := make([]string, 0, len(evs))
	logEntry := &grLogEntry{Type: "data"}
	for _, e := range evs {
		logEntry.Payload = e.Message
		logEntryBytes, err := json.Marshal(logEntry)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		entries = append(entries, string(logEntryBytes))
	}
	return entries, nil
}

func writeEvents(evs []*api.LogEvent, buf *bytes.Buffer) {
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
