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
	"reflect"
	"testing"
	"time"

	"github.com/logrange/logrange/api"
)

func Test_writeEvents(t *testing.T) {
	evs := []*api.LogEvent{{Timestamp: uint64(time.Date(2019, time.January, 1, 1,
		1, 1, 1, time.UTC).UnixNano()),
		Message: "hello\n",
		Fields:  "f1=v1,f2=v2,f3=v3",
	}}

	want := []byte("{\"ts\":\"2019-01-01T01:01:01Z\", \"tags\":\"\", " +
		"\"fields\":\"f1=v1,f2=v2,f3=v3\", \"msg\":\"hello\\n\"}\n")

	got := bytes.Buffer{}
	writeEvents(evs, &got)
	if !reflect.DeepEqual(got.Bytes(), want) {
		t.Errorf("writeEvents() = %v, want %v", string(got.Bytes()), string(want))
	}
}

func TestServer_buildQueryRequest(t *testing.T) {
	type args struct {
		q      string
		pos    string
		limit  int
		offset int
	}
	tests := []struct {
		name string
		args args
		want *api.QueryRequest
	}{
		{
			name: "build tail request ok",
			args: args{q: "file:f1 or pod:p1", pos: "tail", limit: 123, offset: -10},
			want: &api.QueryRequest{
				Limit: 123, Offset: -10, Pos: "tail",
				Query: "SELECT FROM partition WHERE (fields:cid=\"f1\" OR fields:pod=\"p1\") OR " +
					"fields:file CONTAINS \"f1\" OFFSET -10 LIMIT 123",
			},
		},
		{
			name: "build head request ok", args: args{q: "file:f1 or pod:p1", pos: "head", limit: 123, offset: 10},
			want: &api.QueryRequest{
				Limit: 123, Offset: 10, Pos: "head",
				Query: "SELECT FROM partition WHERE (fields:cid=\"f1\" OR fields:pod=\"p1\") OR " +
					"fields:file CONTAINS \"f1\" OFFSET 10 LIMIT 123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				lrPartition: "partition",
			}
			if got := s.buildQueryRequest(tt.args.q,
				tt.args.pos, tt.args.limit, tt.args.offset); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Server.buildQueryRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tarGzEntryWriter_nextEntryHeader(t *testing.T) {
	want := &tar.Header{Name: "prefix.0",
		ModTime: time.Now(), Mode: 0777, Typeflag: tar.TypeReg, Size: 123}

	w := &tarGzEntryWriter{
		entryNum:  1,
		entryPrfx: "prefix",
	}

	got := w.nextEntryHeader(123)
	gtt := got.ModTime.Add(time.Minute)

	want.ModTime = got.ModTime
	if gtt.Before(want.ModTime) {
		t.Errorf("tarGzEntryWriter.nextEntryHeader() = %v, want %v",
			got.ModTime, want.ModTime)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tarGzEntryWriter.nextEntryHeader() = %v, want %v",
			got, want)
	}
}

func Test_tarGzEntryWriter_write(t *testing.T) {
	want := []byte("test")

	//write
	wbuf := bytes.Buffer{}
	gw := gzip.NewWriter(&wbuf)
	tw := tar.NewWriter(gw)
	w := &tarGzEntryWriter{entryNum: 123, entryPrfx: "prefix", gzWriter: gw, tarWriter: tw}
	_ = w.write(want)
	w.close()

	//read
	gzReader, _ := gzip.NewReader(&wbuf)
	tarReader := tar.NewReader(gzReader)
	h, _ := tarReader.Next()
	rbuf := make([]byte, h.Size)
	_, _ = tarReader.Read(rbuf)

	//check
	if !reflect.DeepEqual(rbuf, want) {
		t.Errorf("tarGzEntryWriter.write() = %v, want %v", rbuf, want)
	}
}
