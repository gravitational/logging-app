package main

import (
	"fmt"
	"regexp"
	"strings"
)

func buildMatcher(filter filter) string {
	if filter.freeText != "" {
		return regexp.QuoteMeta(strings.TrimSpace(filter.freeText))
	}
	containerMatcher := "(" + strings.Join(filter.containers, "|") + ")"
	podNamespace := match.placeholder
	var pods []string
	if len(filter.pods) > 0 {
		pods = filter.pods
	} else {
		pods = []string{match.placeholder}
	}

	var podMatchers []string
	for _, podName := range pods {
		podMatchers = append(podMatchers, fmt.Sprintf(`%v_%v_%v`, podName, podNamespace, containerMatcher))
	}
	return "^" + prefix + match.whitespace + "(" + strings.Join(podMatchers, "|") + ")"
}

// Macthers
var match = struct {
	timestamp   string
	whitespace  string
	forwarder   string
	placeholder string
}{
	timestamp:   `[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z`,
	whitespace:  `\s+`,
	forwarder:   `[a-zA-Z\0-9-]+`,
	placeholder: `[^_]+`,
}

// prefix defines a common log prefix pattern relevant for any log entry
var prefix string = match.timestamp + match.whitespace + match.forwarder
