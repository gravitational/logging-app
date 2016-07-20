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
// split it into parts for each search component
func parseQuery(r io.Reader) (filter filter, err error) {
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	d := map[string][]string{}
	var errors []error
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 {
			continue
		}
		p := &parser{}
		p.s.Init(strings.NewReader(line))
		p.next()
		for p.tok != scanner.EOF && len(p.errors) == 0 {
			term := p.parseTerm()
			if p.tok != scanner.EOF {
				p.parseOp() // skip operators
			}
			d[strings.ToLower(term.name)] = append(d[term.name], term.value)
		}
		if len(p.errors) > 0 {
			errors = append(errors, p.errors...)
			break
		}
		filter.containers = append(filter.containers, d["container"]...)
		filter.pods = append(filter.pods, d["pod"]...)
	}
	if len(errors) > 0 {
		return filter, trace.NewAggregate(errors...)
	}
	return filter, nil
}

type filter struct {
	containers []string
	pods       []string
}

type term struct {
	name  string
	value string
}

type parser struct {
	errors []error
	s      scanner.Scanner
	pos    scanner.Position
	tok    rune
	lit    string
}

type operator string

const (
	opAND operator = "and"
)

func (r *parser) next() {
	r.tok = r.s.Scan()
	r.pos = r.s.Position
	r.lit = r.s.TokenText()
}

func (r *parser) parseTerm() *term {
	name := r.parseIdent()
	r.expect(':')
	value := r.parseIndentOrLit()
	return &term{
		name:  name,
		value: value,
	}
}

func (r *parser) parseIdent() string {
	name := r.lit
	r.expect(scanner.Ident)
	return name
}

func (r *parser) parseIndentOrLit() (value string) {
	var err error
	if value, err = strconv.Unquote(r.lit); err != nil {
		value = r.lit
	}
	r.expectOr(scanner.String, scanner.Ident)
	return value
}

func (r *parser) parseOp() operator {
	op := operator(strings.ToLower(r.lit))
	switch op {
	case opAND:
	default:
		r.error(r.pos, fmt.Sprintf("expected an operator but got %v", r.lit))
	}
	r.expect(scanner.Ident)
	return op
}

func (r *parser) expect(tok rune) {
	if r.tok != tok {
		r.error(r.pos, fmt.Sprintf("expected %v but got %v", scanner.TokenString(tok), r.tok))
	}
	r.next()
}

func (r *parser) expectOr(tokens ...rune) {
	if !tokenInSlice(r.tok, tokens) {
		r.error(r.pos, fmt.Sprintf("expected any of %v but got %v", tokenStrings(tokens), r.tok))
	}
	r.next()
}

func (r *parser) error(pos scanner.Position, msg string) {
	r.errors = append(r.errors, trace.Errorf("%v: %v", pos, msg))
}

func tokenStrings(tokens []rune) string {
	var output bytes.Buffer
	for i, tok := range tokens {
		if i > 0 {
			output.WriteByte(',')
		}
		output.WriteString(scanner.TokenString(tok))
	}
	return output.String()
}

func tokenInSlice(token rune, tokens []rune) bool {
	for _, tok := range tokens {
		if tok == token {
			return true
		}
	}
	return false
}
