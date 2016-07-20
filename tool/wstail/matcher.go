package main

import (
	"fmt"
	"strings"
)

func buildMatcher(filter filter) string {
	containerMatcher := "(" + strings.Join(filter.containers, "|") + ")"
	// TODO: for each pod, build sub-pattern with each container inside
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
	return "(" + strings.Join(podMatchers, "|") + ")"
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
