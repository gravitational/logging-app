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

package journal

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/logrange/range/pkg/records"
	"github.com/logrange/range/pkg/records/chunk"
)

type (
	JIterator struct {
		j     Journal
		pos   Pos
		ci    chunk.Iterator
		bkwrd bool
	}
)

func NewJIterator(j Journal) *JIterator {
	return &JIterator{j: j}
}

func (it *JIterator) SetBackward(bkwrd bool) {
	if it.bkwrd == bkwrd {
		return
	}
	it.bkwrd = bkwrd
	if it.ci != nil {
		it.ci.SetBackward(it.bkwrd)
	}
}

func (it *JIterator) Next(ctx context.Context) {
	it.Get(ctx)
	if it.ci != nil {
		it.ci.Next(ctx)
		pos := it.ci.Pos()
		if pos < 0 {
			it.advanceChunk()
			return
		}
		it.pos.Idx = uint32(pos)
	}
}

func (it *JIterator) Get(ctx context.Context) (records.Record, error) {
	err := it.ensureChkIt()
	if err != nil {
		return nil, err
	}

	rec, err := it.ci.Get(ctx)
	for err == io.EOF {
		err = it.advanceChunk()
		if err != nil {
			return nil, err
		}
		rec, err = it.ci.Get(ctx)
	}
	return rec, err
}

func (it *JIterator) CurrentPos() records.IteratorPos {
	return it.Pos()
}

func (it *JIterator) Pos() Pos {
	return it.pos
}

func (it *JIterator) SetPos(pos Pos) {
	if pos == it.pos {
		return
	}

	if pos.CId != it.pos.CId {
		it.closeChunk()
	}

	if it.ci != nil {
		it.ci.SetPos(int64(pos.Idx))
	}
	it.pos = pos
}

func (it *JIterator) Close() error {
	it.closeChunk()
	it.j = nil
	return nil
}

func (it *JIterator) Release() {
	if it.ci != nil {
		it.ci.Release()
	}
}

func (it *JIterator) closeChunk() {
	if it.ci != nil {
		it.ci.Close()
		it.ci = nil
	}
}

func (it *JIterator) advanceChunk() error {
	it.closeChunk()
	if it.bkwrd {
		it.pos.CId--
		it.pos.Idx = math.MaxUint32
	} else {
		it.pos.CId++
		it.pos.Idx = 0
	}
	return it.ensureChkIt()
}

// ensureChkId selects chunk by position JIterator. It corrects the position if needed
func (it *JIterator) ensureChkIt() error {
	if it.ci != nil {
		return nil
	}

	var chk chunk.Chunk
	if it.bkwrd {
		chk = it.getChunkByIdOrLess(it.pos.CId)
	} else {
		chk = it.getChunkByIdOrGreater(it.pos.CId)
	}

	if chk == nil {
		return io.EOF
	}

	if chk.Id() < it.pos.CId {
		it.pos.CId = chk.Id()
		it.pos.Idx = chk.Count()
		if !it.bkwrd {
			return io.EOF
		}
	}

	if chk.Id() > it.pos.CId {
		it.pos.CId = chk.Id()
		it.pos.Idx = 0
	}

	var err error
	it.ci, err = chk.Iterator()
	if err != nil {
		return err
	}
	it.ci.SetBackward(it.bkwrd)
	it.ci.SetPos(int64(it.pos.Idx))
	it.pos.Idx = uint32(it.ci.Pos())
	return nil
}

func (it *JIterator) String() string {
	return fmt.Sprintf("{pos=%s, ci exist=%t}", it.pos, it.ci != nil)
}

func (it *JIterator) getChunkByIdOrGreater(cid chunk.Id) chunk.Chunk {
	chunks, _ := it.j.Chunks().Chunks(context.Background())
	n := len(chunks)
	if n == 0 {
		return nil
	}

	idx := sort.Search(n, func(i int) bool { return chunks[i].Id() >= cid })
	// according to the condition idx is always in [0..n]
	if idx < n {
		return chunks[idx]
	}
	return chunks[n-1]
}

func (it *JIterator) getChunkByIdOrLess(cid chunk.Id) chunk.Chunk {
	chunks, _ := it.j.Chunks().Chunks(context.Background())
	n := len(chunks)
	if n == 0 || chunks[0].Id() > cid {
		return nil
	}

	idx := sort.Search(n, func(i int) bool { return chunks[i].Id() > cid })
	return chunks[idx-1]
}
