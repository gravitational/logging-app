package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/gravitational/trace"
)

// parseQuery interprets the specified reader as a query string and
// splits it into parts for each search component
func parseQuery(r io.Reader) (filter filter, err error) {
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	terms := map[string][]string{}
	var errors []error
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 {
			continue
		}
		p := &parser{}
		p.scanner.Init(strings.NewReader(line))
		p.next()
		for p.token != scanner.EOF && len(p.errors) == 0 {
			term := p.parseTerm()
			if p.token != scanner.EOF {
				p.parseOperator() // skip operators
			}
			terms[strings.ToLower(term.name)] = append(terms[term.name], term.value)
		}
		if len(p.errors) > 0 {
			errors = append(errors, p.errors...)
			break
		}
		filter.containers = append(filter.containers, terms["container"]...)
		filter.pods = append(filter.pods, terms["pod"]...)
	}
	if len(errors) > 0 {
		return filter, trace.NewAggregate(errors...)
	}
	return filter, nil
}

type filter struct {
	containers []string
	pods       []string
	// freeText defines the part of the filter with unstructured text
	freeText string
}

type term struct {
	name  string
	value string
}

type parser struct {
	errors  []error
	scanner scanner.Scanner
	pos     scanner.Position
	token   rune
	literal string
}

type operator string

const (
	opAND operator = "and"
)

func (r *parser) next() {
	r.token = r.scanner.Scan()
	r.pos = r.scanner.Position
	r.literal = r.scanner.TokenText()
}

func (r *parser) parseTerm() *term {
	name := r.parseIdent()
	r.expect(':')
	value := r.parseIndentOrLiteral()
	return &term{
		name:  name,
		value: value,
	}
}

func (r *parser) parseIdent() string {
	name := r.literal
	r.expect(scanner.Ident)
	return name
}

func (r *parser) parseIndentOrLiteral() (value string) {
	var err error
	if value, err = strconv.Unquote(r.literal); err != nil {
		value = r.literal
	}
	r.expectOr(scanner.String, scanner.Ident)
	return value
}

func (r *parser) parseOperator() operator {
	operator := operator(strings.ToLower(r.literal))
	switch operator {
	case opAND:
	default:
		r.error(r.pos, fmt.Sprintf("expected an operator but got %v", r.literal))
	}
	r.expect(scanner.Ident)
	return operator
}

func (r *parser) expect(token rune) {
	if r.token != token {
		r.error(r.pos, fmt.Sprintf("expected %v but got %v", scanner.TokenString(token), scanner.TokenString(r.token)))
	}
	r.next()
}

func (r *parser) expectOr(tokens ...rune) {
	if !tokenInSlice(r.token, tokens) {
		r.error(r.pos, fmt.Sprintf("expected any of %v but got %v", tokenStrings(tokens), scanner.TokenString(r.token)))
	}
	r.next()
}

func (r *parser) error(pos scanner.Position, message string) {
	r.errors = append(r.errors, trace.Errorf("%v: %v", pos, message))
}

func tokenStrings(tokens []rune) string {
	var output bytes.Buffer
	for i, token := range tokens {
		if i > 0 {
			output.WriteByte(',')
		}
		output.WriteString(scanner.TokenString(token))
	}
	return output.String()
}

func tokenInSlice(needle rune, haystack []rune) bool {
	for _, token := range haystack {
		if token == needle {
			return true
		}
	}
	return false
}
