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

package bstorage

import (
	"fmt"
	"github.com/logrange/range/pkg/utils/errors"
)

type (
	inmemBts []byte
)

func NewInMemBytes(size int) Bytes {
	var ib inmemBts
	ib = make([]byte, size)
	return &ib
}

// Close is part of Bytes interface
func (ib *inmemBts) Close() error {
	if *ib == nil {
		return errors.ClosedState
	}
	*ib = nil
	return nil
}

// Size is part of Bytes interface
func (ib *inmemBts) Size() int64 {
	return int64(len(*ib))
}

// Grow is part of Bytes interface
func (ib *inmemBts) Grow(newSize int64) error {
	if ib.isClosed() {
		return errors.ClosedState
	}

	if newSize < ib.Size() {
		return fmt.Errorf("then newSize=%d must be greater than current one, which is %d", newSize, ib.Size())
	}
	nb := make([]byte, newSize)
	copy(nb, *ib)
	*ib = nb
	return nil
}

// Buffer is part of Bytes interface
func (ib *inmemBts) Buffer(offs int64, size int) ([]byte, error) {
	if ib.isClosed() {
		return nil, errors.ClosedState
	}

	if offs < 0 || offs >= ib.Size() {
		return nil, ErrOutOfBounds
	}
	if offs+int64(size) > ib.Size() {
		size = int(ib.Size() - offs)
	}
	return (*ib)[int(offs) : int(offs)+size], nil
}

func (ib *inmemBts) String() string {
	return fmt.Sprintf("inmemBts:{size=%d}", ib.Size())
}

func (ib *inmemBts) isClosed() bool {
	return *ib == nil
}
