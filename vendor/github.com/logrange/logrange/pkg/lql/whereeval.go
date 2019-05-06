// Copyright 2018-2019 The logrange Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lql

import (
	bytes2 "bytes"
	"fmt"
	"github.com/logrange/logrange/pkg/model"
	"github.com/logrange/logrange/pkg/scanner/parser/date"
	"github.com/logrange/range/pkg/utils/bytes"
	"path"
	"strconv"
	"strings"
)

type (
	// WhereExpFunc returns true if the provided LogEvent matches the where condition
	WhereExpFunc func(le *model.LogEvent) bool

	whereExpFuncBuilder struct {
		wef WhereExpFunc
	}
)

var positiveWhereExpFunc = func(*model.LogEvent) bool { return true }

// BuildWhereExpFunc buildw WHERE condition by the condition human readable form like `a=b AND c=d`
func BuildWhereExpFunc(wCond string) (WhereExpFunc, error) {
	exp, err := ParseExpr(wCond)
	if err != nil {
		return nil, err
	}
	return BuildWhereExpFuncByExpression(exp)
}

// BuildWhereExpFuncByExpression builds where function by the Expression provided
func BuildWhereExpFuncByExpression(exp *Expression) (WhereExpFunc, error) {
	if exp == nil {
		return positiveWhereExpFunc, nil
	}

	var web whereExpFuncBuilder
	err := web.buildOrConds(exp.Or)
	if err != nil {
		return nil, err
	}

	return web.wef, nil
}

func (web *whereExpFuncBuilder) buildOrConds(ocn []*OrCondition) error {
	if len(ocn) == 0 {
		web.wef = positiveWhereExpFunc
		return nil
	}

	err := web.buildXConds(ocn[0].And)
	if err != nil {
		return err
	}

	if len(ocn) == 1 {
		// no need to go ahead anymore
		return nil
	}

	efd0 := web.wef
	err = web.buildOrConds(ocn[1:])
	if err != nil {
		return err
	}
	efd1 := web.wef

	web.wef = func(le *model.LogEvent) bool { return efd0(le) || efd1(le) }
	return nil
}

func (web *whereExpFuncBuilder) buildXConds(cn []*XCondition) (err error) {
	if len(cn) == 0 {
		web.wef = positiveWhereExpFunc
		return nil
	}

	if len(cn) == 1 {
		return web.buildXCond(cn[0])
	}

	err = web.buildXCond(cn[0])
	if err != nil {
		return err
	}

	efd0 := web.wef
	err = web.buildXConds(cn[1:])
	if err != nil {
		return err
	}
	efd1 := web.wef

	web.wef = func(le *model.LogEvent) bool { return efd0(le) && efd1(le) }
	return nil

}

func (web *whereExpFuncBuilder) buildXCond(xc *XCondition) (err error) {
	if xc.Expr != nil {
		err = web.buildOrConds(xc.Expr.Or)
	} else {
		err = web.buildCond(xc.Cond)
	}

	if err != nil {
		return err
	}

	if xc.Not {
		efd1 := web.wef
		web.wef = func(le *model.LogEvent) bool { return !efd1(le) }
		return nil
	}

	return nil
}

func (web *whereExpFuncBuilder) buildCond(cn *Condition) (err error) {
	op := strings.ToLower(cn.Operand)
	if op == OPND_TIMESTAMP {
		return web.buildTsCond(cn)
	}

	if op == OPND_MESSAGE {
		return web.buildMsgCond(cn)
	}

	if !strings.HasPrefix(op, "fields:") || len(op) < 8 {
		return fmt.Errorf("operand must be ts, msg, or fields:<fieldname> with non-empty fieldname")
	}
	return web.buildFldCond(cn)
}

func (web *whereExpFuncBuilder) buildTsCond(cn *Condition) error {
	tm, err := parseTime(cn.Value)
	if err != nil {
		return err
	}

	switch cn.Op {
	case "<":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp < tm
		}
	case ">":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp > tm
		}
	case "<=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp <= tm
		}
	case ">=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp >= tm
		}
	case "!=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp != tm
		}
	case "=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Timestamp == tm
		}
	default:
		err = fmt.Errorf("Unsupported operation %s for timetstamp comparison", cn.Op)
	}
	return err
}

func (web *whereExpFuncBuilder) buildMsgCond(cn *Condition) (err error) {
	op := strings.ToUpper(cn.Op)
	val := bytes.StringToByteArray(cn.Value)
	switch op {
	case CMP_CONTAINS:
		web.wef = func(le *model.LogEvent) bool {
			return bytes2.Contains(le.Msg, val)
		}
	case CMP_HAS_PREFIX:
		web.wef = func(le *model.LogEvent) bool {
			return bytes2.HasPrefix(le.Msg, val)
		}
	case CMP_HAS_SUFFIX:
		web.wef = func(le *model.LogEvent) bool {
			return bytes2.HasSuffix(le.Msg, val)
		}
	case CMP_LIKE:
		// test it first
		_, err := path.Match(cn.Value, "abc")
		if err != nil {
			err = fmt.Errorf("Wrong 'like' expression for %s, err=%s", cn.Value, err.Error())
		} else {
			web.wef = func(le *model.LogEvent) bool {
				res, _ := path.Match(cn.Value, le.Msg.AsWeakString())
				return res
			}
		}
	default:
		err = fmt.Errorf("Unsupported operation %s for msg field %s", cn.Op, cn.Operand)
	}
	return err
}

func (web *whereExpFuncBuilder) buildFldCond(cn *Condition) (err error) {
	// the fldName is prefixed by `fields:`, so cut it
	fldName := cn.Operand[7:]
	op := strings.ToUpper(cn.Op)
	switch op {
	case CMP_CONTAINS:
		web.wef = func(le *model.LogEvent) bool {
			return strings.Contains(le.Fields.Value(fldName), cn.Value)
		}
	case CMP_HAS_PREFIX:
		web.wef = func(le *model.LogEvent) bool {
			return strings.HasPrefix(le.Fields.Value(fldName), cn.Value)
		}
	case CMP_HAS_SUFFIX:
		web.wef = func(le *model.LogEvent) bool {
			return strings.HasSuffix(le.Fields.Value(fldName), cn.Value)
		}
	case CMP_LIKE:
		// test it first
		_, err := path.Match(cn.Value, "abc")
		if err != nil {
			err = fmt.Errorf("Uncompilable 'like' expression for \"%s\", expected a shell pattern (not regexp) err=%s", cn.Value, err.Error())
		} else {
			web.wef = func(le *model.LogEvent) bool {
				res, _ := path.Match(cn.Value, le.Fields.Value(fldName))
				return res
			}
		}
	case "=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) == cn.Value
		}
	case "!=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) != cn.Value
		}
	case ">":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) > cn.Value
		}
	case "<":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) < cn.Value
		}
	case ">=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) >= cn.Value
		}
	case "<=":
		web.wef = func(le *model.LogEvent) bool {
			return le.Fields.Value(fldName) <= cn.Value
		}
	default:
		err = fmt.Errorf("Unsupported operation %s for field %s", cn.Op, cn.Operand)
	}
	return err
}

func parseTime(val string) (uint64, error) {
	v, err := strconv.ParseInt(val, 10, 64)
	if err == nil {
		return uint64(v), nil
	}

	tm, err := date.Parse(bytes.StringToByteArray(val))
	if err != nil {
		return 0, err
	}

	return uint64(tm.UnixNano()), nil
}
