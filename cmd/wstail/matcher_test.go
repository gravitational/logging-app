package main

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestMatcher(t *testing.T) { TestingT(t) }

type MatcherSuite struct{}

var _ = Suite(&MatcherSuite{})

func (s *MatcherSuite) TestBuildsMatcher(c *C) {
	var testCases = []struct {
		filter   filter
		expected string
		comment  string
	}{
		{
			filter: filter{
				containers: []string{"foo", "bar"},
			},
			expected: matchPrefix + matchWhitespace + `([^_]+_[^_]+_(foo|bar))`,
			comment:  "combines multiple containers",
		},
		{
			filter: filter{
				containers: []string{"foo"},
				pods:       []string{"qux", "baz"},
			},
			expected: matchPrefix + matchWhitespace + `(qux_[^_]+_(foo)|baz_[^_]+_(foo))`,
			comment:  "replicates containers for each pod",
		},
	}

	for _, testCase := range testCases {
		matcher := buildMatcher(testCase.filter)
		c.Assert(matcher, Equals, testCase.expected, Commentf(testCase.comment))
	}
}
