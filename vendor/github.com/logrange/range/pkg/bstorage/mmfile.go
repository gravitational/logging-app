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
	"github.com/edsrzf/mmap-go"
	"github.com/pkg/errors"
	"os"
)

type (
	// MMFile struct provides file memory mapping and the Bytes interface implementation
	//
	// NOTE: the object is Read-Write go-rouitne safe. It means that the methods Read and
	// Write could be called for not overlapping bytes regions from different go-routines
	// at the same time, but not other methods for the object calls are allowed.
	MMFile struct {
		fn   string
		f    *os.File
		mf   mmap.MMap
		size int64
	}
)

// NewMMFile creates new or open existing and maps a region with the size into map.
// The size region must be multiplied on os.Getpagesize() (4096?). If the file size doesn't exist,
// or its initial size is less than the mapped size provided the file phisical size will be extended to
// the size. If the size is less than 0, than the mapping will try to be done to the actual file size.
func NewMMFile(fname string, size int64) (*MMFile, error) {
	if size < 0 {
		fi, err := os.Stat(fname)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read info for file %s, but mapped size=%d", fname, size)
		}
		size = fi.Size()
	}

	if err := checkSize(size); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open file %s", fname)
	}

	mf, err := mmap.MapRegion(f, int(size), mmap.RDWR, 0, 0)
	if err != nil {
		f.Close()
		return nil, errors.Wrapf(err, "could not memory map file %s", fname)
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, errors.Wrapf(err, "could not read file %s info after mapping ", fname)
	}

	if fi.Size() < size {
		// Extend the file for correct mappin on the macos
		if err = f.Truncate(size); err != nil {
			return nil, errors.Wrapf(err, "could not extend file %s size to %d ", fname, size)
		}
	}

	mmf := new(MMFile)
	mmf.fn = fname
	mmf.f = f
	mmf.mf = mf
	mmf.size = size

	return mmf, nil
}

// Close closes the mapped file
func (mmf *MMFile) Close() error {
	var err error
	if mmf.f != nil {
		mmf.unmap()
		err = mmf.f.Close()
		mmf.f = nil
		mmf.size = -1
	}
	return err
}

// Size returns the size of mapped region
func (mmf *MMFile) Size() int64 {
	return mmf.size
}

// Grow allows to increase the mapped region.
func (mmf *MMFile) Grow(newSize int64) (err error) {
	if mmf.size >= newSize {
		return fmt.Errorf("expecting new size %d to be more the current=%d", newSize, mmf.size)
	}

	if err := checkSize(newSize); err != nil {
		return err
	}

	mmf.unmap()

	if err = mmf.f.Truncate(newSize); err != nil {
		mmf.Close()
		return errors.Wrapf(err, "could not extend file size to %d ", newSize)
	}

	mmf.mf, err = mmap.MapRegion(mmf.f, int(newSize), mmap.RDWR, 0, 0)
	if err != nil {
		mmf.Close()
		return err
	}
	mmf.size = newSize
	return
}

// Buffer returns Mapped memory slice to be read and written.
func (mmf *MMFile) Buffer(offs int64, size int) ([]byte, error) {
	if offs < 0 || offs >= mmf.size {
		return nil, fmt.Errorf("offset=%d out of bounds [0..%d]", offs, mmf.size-1)
	}

	idx := int(offs)
	if idx+size >= int(mmf.size) {
		size = int(mmf.size - offs)
	}

	return mmf.mf[idx : idx+size], nil
}

func (mmf *MMFile) String() string {
	if mmf.f != nil {
		return fmt.Sprintf("MMFile: {fn=%s, f=\"opened\", size=%d}", mmf.fn, mmf.size)
	}
	return fmt.Sprintf("MMFile{fn=%s, f=\"closed\", size=%d}", mmf.fn, mmf.size)
}

func (mmf *MMFile) unmap() {
	if mmf.mf == nil {
		return
	}
	mmf.mf.Unmap()
}

func checkSize(size int64) error {
	if size <= 0 {
		return fmt.Errorf("provided size must be positive, or the file should not be empty, but size=%d", size)
	}

	if size%int64(os.Getpagesize()) != 0 {
		return fmt.Errorf("size=%d must be a multiple of %d", size, os.Getpagesize())
	}
	return nil
}
