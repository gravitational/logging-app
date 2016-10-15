package main

import (
	"path/filepath"

	. "gopkg.in/check.v1"
)

func (s *S) TestOrdersRotatedLogs(c *C) {
	const dir = "foo"
	var testCases = []struct {
		comment  string
		names    []string
		expected rotated
	}{
		{
			comment:  "empty",
			names:    nil,
			expected: rotated{},
		},
		{
			comment: "filters irrelevant files",
			names:   []string{"messages.1.gz", "messages.2.gz", "bar"},
			expected: rotated{
				Main: "",
				Compressed: []string{
					filepath.Join(dir, "messages.1.gz"),
					filepath.Join(dir, "messages.2.gz")},
			},
		},
		{
			comment: "arranges compressed files in proper order",
			names:   []string{"messages.2.gz", "messages.1.gz"},
			expected: rotated{
				Main: "",
				Compressed: []string{
					filepath.Join(dir, "messages.1.gz"),
					filepath.Join(dir, "messages.2.gz")},
			},
		},
		{
			comment: "structures files in properly",
			names:   []string{"messages.0", "messages.2.gz", "messages.1.gz"},
			expected: rotated{
				Main: filepath.Join(dir, "messages.0"),
				Compressed: []string{
					filepath.Join(dir, "messages.1.gz"),
					filepath.Join(dir, "messages.2.gz")},
			},
		},
	}

	for _, testCase := range testCases {
		obtained := newRotated(dir, testCase.names)
		c.Assert(obtained, DeepEquals, testCase.expected, Commentf(testCase.comment))
	}
}
