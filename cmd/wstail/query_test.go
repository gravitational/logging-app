package main

import (
	. "gopkg.in/check.v1"
)

func (r *S) TestParsesQueries(c *C) {
	var testCases = []struct {
		query   string
		result  filter
		err     string
		comment string
	}{
		{
			query: `pod:foo`,
			result: filter{
				pods: []string{"foo"},
			},
			comment: "Parses a term",
		},
		{
			query: `pod:"foo-bar"`,
			result: filter{
				pods: []string{"foo-bar"},
			},
			comment: "Parses a quoted term",
		},
		{
			query: `pod:foo-bar`,
			result: filter{
				pods: []string{"foo-bar"},
			},
			comment: "Parses a complex term",
		},
		{
			query: `pod:foo-bar and container:bar-qux and file:demo.log`,
			result: filter{
				pods:       []string{"foo-bar"},
				containers: []string{"bar-qux"},
				files:      []string{"demo.log"},
			},
			comment: "Parses a complex query with multiple terms",
		},
		{
			query: `this is a free-text search`,
			result: filter{
				freeText: "this is a free-text search",
			},
			err:     ".*expected.*but got.*",
			comment: "Unparsable text becomes free text",
		},
	}

	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		result, err := parseQuery([]byte(testCase.query))
		if testCase.err == "" {
			c.Assert(err, IsNil, comment)
		} else {
			c.Assert(err, ErrorMatches, testCase.err)
		}
		c.Assert(result, DeepEquals, testCase.result, comment)
	}
}
