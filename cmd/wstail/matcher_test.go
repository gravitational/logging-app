package main

import (
	. "gopkg.in/check.v1"
)

func (s *S) TestBuildsMatcher(c *C) {
	var testCases = []struct {
		filter   filter
		expected string
		comment  string
	}{
		{
			filter: filter{
				containers: []string{"foo", "bar"},
			},
			expected: matchPrefix + matchWhitespace + `([^_]+_[^_]+_(foo-|bar-))`,
			comment:  "combines multiple containers",
		},
		{
			filter: filter{
				containers: []string{"foo"},
				pods:       []string{"qux", "baz"},
			},
			expected: matchPrefix + matchWhitespace + `(qux_[^_]+_(foo-)|baz_[^_]+_(foo-))`,
			comment:  "replicates containers for each pod",
		},
		{
			filter: filter{
				files: []string{"foo-bar.log"},
			},
			expected: matchPrefix + matchWhitespace + `([^_]+_[^_]+_([a-zA-Z\0-9-]+-foo-bar.log))|foo-bar.log`,
			comment:  "matches a file either in pod/container context or alone",
		},
	}

	for _, testCase := range testCases {
		matcher := buildMatcher(testCase.filter)
		c.Assert(matcher, Equals, testCase.expected, Commentf(testCase.comment))
	}
}
