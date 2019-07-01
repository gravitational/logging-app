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

/*
 bstorage package provides Bytes interface for accessing to a byte-storage.
 It can be used for building data indexes on top of filesystem and store trees into
 a file via the Bytes interface.
*/
package bstorage

import "io"

type (
	// Bytes interface provides an access to a byte storage
	Bytes interface {
		io.Closer

		// Size returns the current storage size.
		Size() int64

		// Grow allows to increase the storage size.
		Grow(newSize int64) error

		// Buffer returns a segment of the Bytes storage as a the slice.
		// The slice can be read and written. The data written to the slice will be
		// saved in the Bytes buffer. Different goroutines are allowed
		// to request not overlapping buffers. Writing to the same segment
		// from different go routines causes unpredictable result.
		Buffer(offs int64, size int) ([]byte, error)
	}
)
