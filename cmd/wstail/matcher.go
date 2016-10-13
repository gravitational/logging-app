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
	var suffixMatches []string
	var containers = filter.containers
	if len(containers) == 0 {
		containers = []string{matchNamePlaceholder}
	}
	var files = filter.files
	if len(files) == 0 {
		files = []string{""} // Empty placeholder if no file filter specified
	}
	for _, file := range files {
		for _, container := range containers {
			suffixMatches = append(suffixMatches, fmt.Sprintf("%v-%v", container, file))
		}
	}
	containerFileMatcher := "(" + strings.Join(suffixMatches, "|") + ")"

	podNamespace := matchPlaceholder
	var pods []string
	if len(filter.pods) > 0 {
		pods = filter.pods
	} else {
		pods = []string{matchPlaceholder}
	}

	var podMatchers []string
	for _, podName := range pods {
		podMatchers = append(podMatchers, fmt.Sprintf(`%v_%v_%v`, podName, podNamespace, containerFileMatcher))
	}
	capture := []string{"(" + strings.Join(podMatchers, "|") + ")"}
	// In case of a file filter, also consider files outside of k8s context
	for _, file := range filter.files {
		capture = append(capture, file)
	}

	return matchPrefix + matchWhitespace + strings.Join(capture, "|")
}

// Matchers
const (
	matchTimestamp       = `[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z`
	matchWhitespace      = `[[:space:]]+`
	matchNamePlaceholder = `[a-zA-Z\0-9-]+`
	matchPlaceholder     = `[^_]+`
)

// matchPrefix defines a common log prefix pattern relevant for any log entry
var matchPrefix string = "^" + matchTimestamp + matchWhitespace + matchNamePlaceholder
