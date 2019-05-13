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

package query

import (
	"github.com/logrange/logrange/pkg/lql"
	"testing"
)

func Test_BuildLqlQuery(t *testing.T) {
	type args struct {
		grQuery string
		pipe    string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "build query from empty string ok",
			args: args{
				grQuery: "",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__"},

		{name: "build query with literal search ok",
			args: args{
				grQuery: "POD:pd1 AND podmist\"ake",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE msg CONTAINS \"POD:pd1 AND podmist\\\"ake\""},

		{name: "build query with different case ok",
			args: args{
				grQuery: "POD:po1 or NOT pod:\"pod2\" and file:\"file1\" AND file:\"file2\" and noT (container:\"container1\" or container:cnt2)",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE (fields:pod=\"po1\" OR (NOT fields:pod=\"pod2\" AND fields:cid=\"file1\" AND fields:cid=\"file2\" AND NOT (fields:cname=\"container1\" OR fields:cname=\"cnt2\"))) OR fields:file CONTAINS \"file1\" OR fields:file CONTAINS \"file2\""},

		{name: "build query with file condition ok",
			args: args{
				grQuery: "(NOT POD:pd1) AND File:fLe1",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE (NOT fields:pod=\"pd1\" AND fields:cid=\"fLe1\") OR fields:file CONTAINS \"fLe1\""},

		{name: "build query with escaping ok",
			args: args{
				grQuery: "pod:\"p\\\\d1\"",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE fields:pod=\"p\\\\d1\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildLqlQuery(tt.args.grQuery, tt.args.pipe, 0, 0)
			if got != tt.want {
				t.Errorf("BuildLqlQuery() = %v, want %v", got, tt.want)
			}

			if _, err := lql.ParseLql(got); err != nil {
				t.Errorf("BuildLqlQuery() = %v, err= %v", got, err)
			}
		})
	}
}
