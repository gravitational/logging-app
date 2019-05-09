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

func Test_BuildTailLqlQuery(t *testing.T) {
	type args struct {
		grQuery string
		pipe    string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "test1",
			args: args{
				grQuery: "",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ POSITION TAIL"},

		{name: "test2",
			args: args{
				grQuery: "POD:pd1 AND podmist\"ake",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE msg CONTAINS \"POD:pd1 AND podmist\\\"ake\" POSITION TAIL"},

		{name: "test3",
			args: args{
				grQuery: "POD:po1 or NOT pod:\"pod2\" and file:\"file1\" AND file:\"file2\" and noT (container:\"container1\" or container:cnt2)",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE (fields:pod=\"po1\" OR (NOT fields:pod=\"pod2\" AND fields:cid=\"file1\" AND fields:cid=\"file2\" AND NOT (fields:cname=\"container1\" OR fields:cname=\"cnt2\"))) OR fields:file CONTAINS \"file1\" OR fields:file CONTAINS \"file2\" POSITION TAIL"},

		{name: "test4",
			args: args{
				grQuery: "(NOT POD:pd1) AND File:fLe1",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE (NOT fields:pod=\"pd1\" AND fields:cid=\"fLe1\") OR fields:file CONTAINS \"fLe1\" POSITION TAIL"},

		{name: "test5",
			args: args{
				grQuery: "pod:\"p\\\\d1\"",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ WHERE fields:pod=\"p\\\\d1\" POSITION TAIL"},

		{name: "test6",
			args: args{
				grQuery: "",
				pipe:    "logrange.pipe=__default__",
			},
			want: "SELECT FROM logrange.pipe=__default__ POSITION TAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTailLqlQuery(tt.args.grQuery, tt.args.pipe, 0, 0)
			if got != tt.want {
				t.Errorf("BuildTailLqlQuery() = %v, want %v", got, tt.want)
			}

			if _, err := lql.ParseLql(got); err != nil {
				t.Errorf("BuildTailLqlQuery() = %v, err= %v", got, err)
			}
		})
	}
}
