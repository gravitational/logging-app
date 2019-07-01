/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package query

import (
	"bytes"
	"fmt"
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"strconv"
	"strings"
)

type (
	// Represents parsed 'Gravity log query'
	query struct {
		Exp *expression `parser:"@@"`
	}

	condition struct {
		Key   string `parser:"@(\"POD\"|\"CONTAINER\"|\"FILE\") \":\""`
		Value string `parser:"@(String|Ident)"`
	}

	expression struct {
		Or []*orCondition `parser:"@@ { \"OR\" @@ }"`
	}

	orCondition struct {
		And []*xCondition `parser:"@@ { \"AND\" @@ }"`
	}

	xCondition struct {
		Not  bool        `parser:"(@\"NOT\")?"`
		Cond *condition  `parser:"(@@"`
		Expr *expression `parser:"| \"(\" @@ \")\")"`
	}
)

var (
	// Gravity log query lexer
	qLexer = lexer.Must(lexer.Regexp(`(\s+)` +
		`|(?P<Keyword>(?i)POD|CONTAINER|FILE|AND|OR|NOT)` +
		`|(?P<Ident>[a-zA-Z0-9_\.][a-zA-Z0-9_\.\-]*)` +
		`|(?P<Operator>:|[()])` +
		`|(?P<String>"([^\\"]|\\.)*"|'[^']*')`,
	))

	// Gravity log query parser
	qParser = participle.MustBuild(&query{},
		participle.Lexer(qLexer),
		participle.Unquote("String"),
		participle.CaseInsensitive("Keyword"))

	// Map is used to map 'Gravity log terms' to 'Logrange fields'.
	// In Logrange database the 'fields' are attached to each log entry and contain
	// such data as pod name, container name etc. The 'fields' are used in
	// LQL (Logrange Query Language) queries.
	keyToField = map[string]string{
		"POD":       "pod",
		"CONTAINER": "cname",
		"FILE":      "cid",
	}

	escaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
)

func parseGravityQuery(qs string) (*query, error) {
	q := &query{}
	err := qParser.ParseString(qs, q)
	return q, err
}

// The function is used to build LQL (Logrange Query Language) query
// by the given params, basically it translates 'Gravity log query'
// to LQL query.
//
// The result is the valid LQL query which looks for matching entries
// (for given offset and limit). If 'Gravity log query' turns out to be invalid,
// it is used as literal text in LQL query.
func BuildLqlQuery(grQuery string, partition string, limit int, offset int) string {
	var lql bytes.Buffer
	lql.WriteString("SELECT FROM ")
	lql.WriteString(partition)

	if grQuery != "" {
		lql.WriteString(" WHERE ")
		q, err := parseGravityQuery(grQuery)

		if err != nil { // Bad query or literal search request
			lql.WriteString("lower(msg)")
			lql.WriteString(" CONTAINS ")
			lql.WriteString("\"" + strings.ToLower(escaper.Replace(grQuery)) + "\"")

		} else { // Good query
			if len(q.Exp.Or) > 1 {
				lql.WriteString("(")
			}
			var files []string
			lql.WriteString(buildOrLql(q.Exp.Or, &files))
			if len(q.Exp.Or) > 1 {
				lql.WriteString(")")
			}
			for _, f := range files { // Unconditionally add files which match condition
				lql.WriteString(" OR ")
				lql.WriteString(fmt.Sprintf("fields:file CONTAINS \"%v\"", f))
			}
		}
	}

	if offset != 0 {
		lql.WriteString(" OFFSET ")
		lql.WriteString(strconv.Itoa(offset))
	}
	if limit > 0 {
		lql.WriteString(" LIMIT ")
		lql.WriteString(strconv.Itoa(limit))
	}

	return lql.String()
}

func buildOrLql(cnd []*orCondition, files *[]string) string {
	var orLql bytes.Buffer

	for _, c := range cnd {
		if orLql.Len() > 1 {
			orLql.WriteString(" OR ")
		}
		if len(c.And) > 1 {
			orLql.WriteString("(")
		}
		orLql.WriteString(buildAndLql(c.And, files))
		if len(c.And) > 1 {
			orLql.WriteString(")")
		}
	}

	return orLql.String()
}

func buildAndLql(cnd []*xCondition, files *[]string) string {
	var andLql bytes.Buffer

	for _, c := range cnd {
		if andLql.Len() > 1 {
			andLql.WriteString(" AND ")
		}
		if c.Not {
			andLql.WriteString("NOT ")
		}
		if c.Expr != nil {
			if len(c.Expr.Or) > 1 {
				andLql.WriteString("(")
			}
			andLql.WriteString(buildOrLql(c.Expr.Or, files))
			if len(c.Expr.Or) > 1 {
				andLql.WriteString(")")
			}
		} else {
			k, v := strings.ToUpper(c.Cond.Key), escaper.Replace(c.Cond.Value)
			andLql.WriteString(fmt.Sprintf("fields:%v=\"%v\"", keyToField[k], v))
			if k == "FILE" {
				// collect all the files, so that we can add them to the whole query later
				*files = append(*files, v)
			}
		}
	}

	return andLql.String()
}
