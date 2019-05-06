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
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/logrange/logrange/api"
)

func Test_marshal(t *testing.T) {
	type args struct {
		evs []*api.LogEvent
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "test1", args: args{
			evs: []*api.LogEvent{
				{
					Timestamp: uint64(time.Date(2019, time.January, 1, 1,
						1, 1, 1, time.UTC).UnixNano()),
					Message: "hello\n",
					Fields:  "f1=v1,f2=v2,f3=v3",
				},
			},
		}, want: []byte("{\"ts\":\"2019-01-01T01:01:01Z\", \"tags\":\"\", \"fields\":\"f1=v1,f2=v2,f3=v3\", \"msg\":\"hello\\n\"}\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bytes.Buffer{}
			marshal(tt.args.evs, &got)
			if !reflect.DeepEqual(got.Bytes(), tt.want) {
				t.Errorf("marshal() = %v, want %v", string(got.Bytes()), string(tt.want))
			}
		})
	}
}
