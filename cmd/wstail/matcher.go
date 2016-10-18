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

	// Wrap into brackets to preserve context if file filters are present
	var suffix string
	if len(filter.files) > 0 {
		suffix = "(" + strings.Join(capture, "|") + ")"
	} else {
		suffix = strings.Join(capture, "|")
	}

	return matchPrefix + matchWhitespace + suffix
}

// Matchers
const (
	matchTimestamp       = `[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]+Z`
	matchWhitespace      = `[[:space:]]+`
	matchNamePlaceholder = `[a-zA-Z0-9-]+`
	matchPlaceholder     = `[^_]+`
)

// matchPrefix defines a common log prefix pattern relevant for any log entry
var matchPrefix string = "^" + matchTimestamp + matchWhitespace + matchNamePlaceholder
