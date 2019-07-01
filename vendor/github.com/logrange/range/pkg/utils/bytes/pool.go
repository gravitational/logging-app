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

package bytes

import (
	"sync"
)

// Pool manages a pool of slices of bytes. It is supposed to use the pool in
// highly loaded apps where slice of bytes are needed often. An example could be
// handling a network packages.
type Pool struct {
}

var (
	bucketSize = []int{100, 200, 500, 1000, 2000, 5000, 10000, 20000, 40000, 80000, 150000, 300000, 600000, 1000000, 1500000, 3000000}
	pools      []*sync.Pool
)

// init intitializes pools, which is shared between all Pool objects so far
func init() {
	pools = make([]*sync.Pool, len(bucketSize))
	for i, v := range bucketSize {
		// to use new variable inside the New function
		v1 := v
		pools[i] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, v1)
			},
		}
	}
}

// Arrange returns a slice by the size requested. The function could return the slice
// with capacity bigger than the size provided.
func (p *Pool) Arrange(size int) []byte {
	if size == 0 {
		return nil
	}

	if size > bucketSize[len(bucketSize)-1] {
		return make([]byte, size)
	}

	res := pools[getBucket(size)].Get().([]byte)

	// Adjusting result to the requested size, just in case
	return res[:size]
}

// Release allows to place the buffer to appropriate pool for next usage
func (p *Pool) Release(buf []byte) {
	buf = buf[:cap(buf)]
	if len(buf) > bucketSize[len(bucketSize)-1] || len(buf) < bucketSize[0] {
		// disregard very small and very big (> 3M buffers)
		return
	}

	// buf with any size could be released. Look for proper bucket to be sure
	// the buf could be reused out from there.
	idx := getBucket(len(buf))
	if bucketSize[idx] > len(buf) {
		idx--
	}

	pools[idx].Put(buf)
}

func getBucket(size int) int {
	i, j := 0, len(bucketSize)
	for i < j {
		h := int(uint(i+j) >> 1)
		if bucketSize[h] < size {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}
