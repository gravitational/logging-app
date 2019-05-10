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
	type args struct {
		evs []*api.LogEvent
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "test1",
			args: args{evs: []*api.LogEvent{
				{Timestamp: uint64(time.Date(2019, time.January, 1, 1,
					1, 1, 1, time.UTC).UnixNano()),
					Message: "hello\n",
					Fields:  "f1=v1,f2=v2,f3=v3",
				},
			}},
			want: []byte("{\"ts\":\"2019-01-01T01:01:01Z\", \"tags\":\"\", " +
				"\"fields\":\"f1=v1,f2=v2,f3=v3\", \"msg\":\"hello\\n\"}\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bytes.Buffer{}
			writeEvents(tt.args.evs, &got)
			if !reflect.DeepEqual(got.Bytes(), tt.want) {
				t.Errorf("writeEvents() = %v, want %v", string(got.Bytes()), string(tt.want))
			}
		})
	}
}

func TestServer_buildQueryRequest(t *testing.T) {
	type args struct {
		q     string
		limit int
	}
	tests := []struct {
		name string
		args args
		want *api.QueryRequest
	}{
		{name: "test1", args: args{q: "file:f1 or pod:p1", limit: 123},
			want: &api.QueryRequest{
				Limit: 123, Offset: -123, Pos: "tail",
				Query: "SELECT FROM partition WHERE (fields:cid=\"f1\" OR fields:pod=\"p1\") OR " +
					"fields:file CONTAINS \"f1\" POSITION TAIL OFFSET -123 LIMIT 123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				lrPartition: "partition",
			}
			if got := s.buildQueryRequest(tt.args.q, tt.args.limit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Server.buildQueryRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tarGzEntryWriter_nextEntryHeader(t *testing.T) {
	type fields struct {
		entryNum  int
		entryPrfx string
	}
	type args struct {
		size int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *tar.Header
	}{
		{name: "test1",
			fields: fields{entryNum: 1, entryPrfx: "prefix"},
			args:   args{size: 123},
			want: &tar.Header{Name: "prefix.0",
				ModTime: time.Now(), Mode: 0777, Typeflag: tar.TypeReg, Size: 123}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &tarGzEntryWriter{
				entryNum:  tt.fields.entryNum,
				entryPrfx: tt.fields.entryPrfx,
			}

			got := w.nextEntryHeader(tt.args.size)
			gtt := got.ModTime.Add(time.Minute)

			tt.want.ModTime = got.ModTime
			if gtt.Before(tt.want.ModTime) {
				t.Errorf("tarGzEntryWriter.nextEntryHeader() = %v, want %v",
					got.ModTime, tt.want.ModTime)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tarGzEntryWriter.nextEntryHeader() = %v, want %v",
					got, tt.want)
			}
		})
	}
}

func Test_tarGzEntryWriter_write(t *testing.T) {
	wantData := []byte("test")

	t.Run("test1", func(t *testing.T) {
		//write
		wbuf := bytes.Buffer{}
		gw := gzip.NewWriter(&wbuf)
		tw := tar.NewWriter(gw)
		w := &tarGzEntryWriter{entryNum: 123, entryPrfx: "prefix", gzWriter: gw, tarWriter: tw}
		_ = w.write(wantData)
		w.close()

		//read
		gzReader, _ := gzip.NewReader(&wbuf)
		tarReader := tar.NewReader(gzReader)
		h, _ := tarReader.Next()
		rbuf := make([]byte, h.Size)
		_, _ = tarReader.Read(rbuf)

		//check
		if !reflect.DeepEqual(rbuf, wantData) {
			t.Errorf("tarGzEntryWriter.write() = %v, want %v", rbuf, wantData)
		}
	})
}
