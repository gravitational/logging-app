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
	podNamespace := matchPlaceholder
	var pods []string
	if len(filter.pods) > 0 {
		pods = filter.pods
	} else {
		pods = []string{matchPlaceholder}
	}

	var podMatchers []string
	for _, podName := range pods {
		podMatchers = append(podMatchers, fmt.Sprintf(`%v_%v_%v`, podName, podNamespace, containerMatcher))
	}
	return matchPrefix + matchWhitespace + "(" + strings.Join(podMatchers, "|") + ")"
}

// Matchers
const matchTimestamp = `[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z`
const matchWhitespace = `[[:space:]]+`
const matchForwarder = `[a-zA-Z\0-9-]+`
const matchPlaceholder = `[^_]+`

// matchPrefix defines a common log prefix pattern relevant for any log entry
var matchPrefix string = "^" + matchTimestamp + matchWhitespace + matchForwarder
