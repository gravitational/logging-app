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
	"fmt"
	"github.com/logrange/logrange/pkg/model/tag"
	"path"
	"strings"
)

type (
	// TagsExpFunc returns true if the provided tag are matched with the expression
	TagsExpFunc func(tags tag.Set) bool

	tagsExpFuncBuilder struct {
		tef TagsExpFunc
	}

	tagsCondExp struct {
		tags tag.Set
	}
)

var PositiveTagsExpFunc = func(tag.Set) bool { return true }

// BuildTagsExpFuncByCond receives a condition line and parses it to the TagsExpFunc
// The tagCond could be provided in one of the following 2 forms:
//	- conditions like: name=app1 and ip like '123.*'
// 	- tag-line like: {name=app1,ip=123.46.32.44}
func BuildTagsExpFunc(tagsCond string) (TagsExpFunc, error) {
	src, err := ParseSource(tagsCond)
	if err != nil {
		return nil, err
	}

	return BuildTagsExpFuncBySource(src)
}

// BuildTagsExpFuncBySource
func BuildTagsExpFuncBySource(src *Source) (TagsExpFunc, error) {
	if src == nil {
		return PositiveTagsExpFunc, nil
	}

	if src.Tags != nil {
		tc := &tagsCondExp{src.Tags.Tags}
		return tc.subsetOf, nil
	}

	return buildTagsExpFunc(src.Expr)
}

func (tc *tagsCondExp) subsetOf(tags tag.Set) bool {
	return tc.tags.SubsetOf(tags)
}

// buildTagsExpFunc returns  TagsExpFunc by the expression provided
func buildTagsExpFunc(exp *Expression) (TagsExpFunc, error) {
	if exp == nil {
		return PositiveTagsExpFunc, nil
	}

	var teb tagsExpFuncBuilder
	err := teb.buildOrConds(exp.Or)
	if err != nil {
		return nil, err
	}

	return teb.tef, nil
}

func (teb *tagsExpFuncBuilder) buildOrConds(ocn []*OrCondition) error {
	if len(ocn) == 0 {
		teb.tef = PositiveTagsExpFunc
		return nil
	}

	err := teb.buildXConds(ocn[0].And)
	if err != nil {
		return err
	}

	if len(ocn) == 1 {
		// no need to go ahead anymore
		return nil
	}

	efd0 := teb.tef
	err = teb.buildOrConds(ocn[1:])
	if err != nil {
		return err
	}
	efd1 := teb.tef

	teb.tef = func(tags tag.Set) bool { return efd0(tags) || efd1(tags) }
	return nil
}

func (teb *tagsExpFuncBuilder) buildXConds(cn []*XCondition) (err error) {
	if len(cn) == 0 {
		teb.tef = PositiveTagsExpFunc
		return nil
	}

	if len(cn) == 1 {
		return teb.buildXCond(cn[0])
	}

	err = teb.buildXCond(cn[0])
	if err != nil {
		return err
	}

	efd0 := teb.tef
	err = teb.buildXConds(cn[1:])
	if err != nil {
		return err
	}
	efd1 := teb.tef

	teb.tef = func(tags tag.Set) bool { return efd0(tags) && efd1(tags) }
	return nil

}

func (teb *tagsExpFuncBuilder) buildXCond(xc *XCondition) (err error) {
	if xc.Expr != nil {
		err = teb.buildOrConds(xc.Expr.Or)
	} else {
		err = teb.buildTagCond(xc.Cond)
	}

	if err != nil {
		return err
	}

	if xc.Not {
		efd1 := teb.tef
		teb.tef = func(tags tag.Set) bool { return !efd1(tags) }
		return nil
	}

	return nil
}

func (teb *tagsExpFuncBuilder) buildTagCond(cn *Condition) (err error) {
	tvf, err := buildTagIdent(cn.Ident)
	if err != nil {
		return err
	}

	op := strings.ToUpper(cn.Op)
	switch op {
	case "<":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) < cn.Value
		}
	case ">":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) > cn.Value
		}
	case "<=":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) <= cn.Value
		}
	case ">=":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) >= cn.Value
		}
	case "!=":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) != cn.Value
		}
	case "=":
		teb.tef = func(tags tag.Set) bool {
			return tvf(tags) == cn.Value
		}
	case CMP_LIKE:
		// test it first
		_, err := path.Match(cn.Value, "abc")
		if err != nil {
			err = fmt.Errorf("Wrong 'like' expression for %s, err=%s", cn.Value, err.Error())
		} else {
			teb.tef = func(tags tag.Set) bool {
				res, _ := path.Match(cn.Value, tvf(tags))
				return res
			}
		}
	case CMP_CONTAINS:
		teb.tef = func(tags tag.Set) bool {
			return strings.Contains(tvf(tags), cn.Value)
		}
	case CMP_HAS_PREFIX:
		teb.tef = func(tags tag.Set) bool {
			return strings.HasPrefix(tvf(tags), cn.Value)
		}
	case CMP_HAS_SUFFIX:
		teb.tef = func(tags tag.Set) bool {
			return strings.HasSuffix(tvf(tags), cn.Value)
		}
	default:
		err = fmt.Errorf("Unsupported operation %s for '%s' identifier ", cn.Op, cn.Ident.String())
	}
	return err
}

type tagValueF func(tags tag.Set) string

func buildTagIdent(id *Identifier) (tagValueF, error) {
	if len(id.Params) == 0 {
		return func(tags tag.Set) string {
			return tags.Tag(id.Operand)
		}, nil
	}

	if len(id.Params) != 1 {
		return nil, fmt.Errorf("the function UPPER() expects, one parameter, but %d provided", len(id.Params))
	}

	fint, err := buildTagIdent(id.Params[0])
	if err != nil {
		return nil, err
	}

	fn := strings.ToUpper(id.Operand)
	switch fn {
	case "UPPER":
		return func(tags tag.Set) string {
			return strings.ToUpper(fint(tags))
		}, nil
	case "LOWER":
		return func(tags tag.Set) string {
			return strings.ToLower(fint(tags))
		}, nil
	}

	return nil, fmt.Errorf("unexpected function %s", id.Operand)
}
