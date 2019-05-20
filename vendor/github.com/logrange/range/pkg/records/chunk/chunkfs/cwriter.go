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

package chunkfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/logrange/range/pkg/records"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jrivets/log4g"
	"github.com/logrange/range/pkg/utils/errors"
	"github.com/logrange/range/pkg/utils/fileutil"
)

type (
	// cWriter supports writing to a file-chunk. The implementation controls
	// fWriter lifecycle for accessing to the file. Only one goroutine can
	// write into the file at a time. So as the implementation uses fWriter,
	// which has a buffer, it tracks position of confirmed write (synced) records
	// positions to the file and unconfirmed (lro) last record, which is written
	// but not flushed to the sile yet. For to be read throug the file access,
	// any reader should consider lroCfrmd value as last record, because all other
	// ones can be not synced yet, so could be read inconsistent
	//
	// The cWriter has 2 timers - idle and flush. The idle timeout allows to
	// close underlying file descriptor (fWriter) if no write operation happens
	// in the timeout period. The flush timeout allows to flush buffer to the
	// disk in the period of time after last write if it is needed.
	cWrtier struct {
		lock sync.Mutex
		// cntCfrmd is confirmed number of records
		cntCfrmd uint32
		// cnt contains dirty number of records int the chunk
		cnt uint32

		// confirmed file size
		sizeCfrmd int64
		// unconfirmed file size
		size int64

		fileName  string
		w         *fWriter
		iw        *fWriter
		wSgnlChan chan bool

		// closed flag indicates wht cWriter is closed
		closed int32

		logger log4g.Logger

		// writer stuff
		offsBuf []byte
		rhBuf   []byte

		// idle timeout (to close the writer)
		idleTO time.Duration
		// flush timeout
		flushTO time.Duration
		// the maximum chunk size
		maxSize int64

		// flush callback
		onFlushF func()
	}
)

func doNothing() {}

func newCWriter(fileName string, size, maxSize int64, count uint32) *cWrtier {
	if size < 0 {
		panic("size must be 0 or positive")
	}

	cw := new(cWrtier)
	cw.fileName = fileName
	cw.cntCfrmd = count
	cw.cnt = count
	cw.size = size
	cw.sizeCfrmd = size
	cw.offsBuf = make([]byte, ChnkIndexRecSize)
	cw.rhBuf = make([]byte, ChnkDataHeaderSize)
	cw.idleTO = ChnkWriterIdleTO
	cw.flushTO = ChnkWriterFlushTO
	cw.maxSize = maxSize
	cw.onFlushF = doNothing
	cw.logger = log4g.GetLogger("chunk.writer").WithId("{" + fileName + "}").(log4g.Logger)
	return cw
}

func (cw *cWrtier) ensureFWriter() error {
	if atomic.LoadInt32(&cw.closed) != 0 {
		return errors.ClosedState
	}

	var err error
	if cw.w == nil {
		cw.logger.Debug("creating new file-writer")
		cw.w, err = newFWriter(cw.fileName, ChnkWriterBufSize)
		if err != nil {
			return err
		}

		// put 100 to be sure there is a buffer for not blocking signaling routine
		cw.wSgnlChan = make(chan bool, 100)

		cw.iw, err = newFWriter(fileutil.SetFileExt(cw.fileName, ChnkIndexExt), ChnkWriterBufSize)
		if err != nil {
			cw.closeFWritersUnsafe()
			return err
		}

		if cw.count() != cw.cntCfrmd {
			cw.logger.Error("Could not open index file. It's size is different that the count provided count=", cw.count(), ", cw=", cw)
			cw.closeFWritersUnsafe()
			return ErrCorruptedData
		}

		go func(sc chan bool) {
			tmr := time.NewTimer(cw.idleTO)
			if !tmr.Stop() {
				<-tmr.C

			}
			for {
				for !cw.isFlushNeeded() {
					cnt := atomic.LoadUint32(&cw.cnt)
					tmr.Reset(cw.idleTO)
					select {
					case <-tmr.C:
						cw.lock.Lock()
						// check whether lro was advanced while it was sleeping
						if atomic.LoadUint32(&cw.cnt) == cnt {
							cw.logger.Debug("closing file-writer due to idle timeout")
							cw.closeFWritersUnsafe()
							cw.lock.Unlock()
							return
						}
						cw.lock.Unlock()
					case _, ok := <-sc:
						if !tmr.Stop() {
							<-tmr.C
						}
						if !ok {
							// the channel closed
							return
						}
					}
				}

				tmr.Reset(cw.flushTO)
				select {
				case <-tmr.C:
					cw.flush()
				case _, ok := <-sc:
					if !tmr.Stop() {
						<-tmr.C
					}

					if !ok {
						return
					}
				}
			}
		}(cw.wSgnlChan)
	}
	return nil
}

// count returns number of unconfirmed count.
func (cw *cWrtier) count() uint32 {
	if cw.iw != nil {
		return uint32(cw.iw.size() / ChnkIndexRecSize)
	}
	return atomic.LoadUint32(&cw.cnt)
}

// isFlushNeeded returns whether the write buffer (see fWrtier) should be be
// flushed or not
func (cw *cWrtier) isFlushNeeded() bool {
	return atomic.LoadUint32(&cw.cnt) != atomic.LoadUint32(&cw.cntCfrmd)
}

// write receives an iterator and writes records to the file-chunk.
//
// the write returns number of records written, number of the last written
// record (unconfirmed count) and an error if any. It can return an error
// together with non-zero first two parameters, which will indicate that some
// data was written.
//
// It will return no error if iterator is empty (the iterator returns io.EOF)
//
// The function holds lock, so it guarantees that only one go-routine can write into the
// chunk. Holding the lock is made from the performance prospective,
// so it checks whether the writer is closed after every record is written.
// See Close(), which sets the flag without requesting the lock.
//
// the write procedure happens in the context of ctx. Which is used for getting
// records from the iterator.
func (cw *cWrtier) write(ctx context.Context, it records.Iterator) (int, uint32, error) {
	cw.lock.Lock()

	if cw.size >= cw.maxSize {
		cw.lock.Unlock()
		return 0, cw.cnt, errors.MaxSizeReached
	}

	err := cw.ensureFWriter()
	if err != nil {
		cw.lock.Unlock()
		return 0, cw.cnt, err
	}

	// indicates that flush signal already issued
	signaled := cw.isFlushNeeded()
	clsd := false
	var wrtn int
	// checking the closed flag holding cw.lock, allows us to detect Close()
	// call and give up before we iterated completely over the iterator
	for atomic.LoadInt32(&cw.closed) == 0 {
		if cw.size >= cw.maxSize {
			err = errors.MaxSizeReached
			break
		}

		var rec records.Record
		rec, err = it.Get(ctx)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		offs := uint64(cw.w.size())

		// writing the record size -> data chunk
		binary.BigEndian.PutUint32(cw.rhBuf, uint32(len(rec)))
		_, err = cw.w.write(cw.rhBuf)
		if err != nil {
			// close chunk (unrecoverable error)
			cw.logger.Error("Could not write record size to the data chunk. err=", err)
			clsd = true
			break
		}

		// writing the record payload -> data chunk
		_, err = cw.w.write(rec)
		if err != nil {
			// close chunk (unrecoverable error)
			cw.logger.Error("Could not write a record payload. err=", err)
			clsd = true
			break
		}

		// writing the record offset -> index
		binary.BigEndian.PutUint64(cw.offsBuf, offs)
		_, err = cw.iw.write(cw.offsBuf)
		if err != nil {
			// close chunk (unrecoverable error)
			cw.logger.Error("Could not write record offset to the index. err=", err)
			clsd = true
			break
		}

		// update dynamic pararms
		atomic.AddUint32(&cw.cnt, 1)
		atomic.StoreInt64(&cw.size, cw.w.fdPos)

		it.Next(ctx)
		wrtn++

	}

	callOnFlush := false
	if atomic.LoadInt32(&cw.closed) != 0 {
		err = errors.ClosedState
	} else if clsd {
		cw.closeUnsafe()
	} else if cw.w.buffered() == 0 && cw.iw.buffered() == 0 {
		// ok, write buffer is empty, no flush is needed
		cnt := cw.cnt
		atomic.StoreUint32(&cw.cntCfrmd, cw.cnt)
		atomic.StoreInt64(&cw.sizeCfrmd, cw.size)
		callOnFlush = cw.cntCfrmd != cnt
	} else if !signaled {
		// signal the channel about write anyway
		cw.wSgnlChan <- true
	}

	cw.lock.Unlock()
	if callOnFlush {
		cw.onFlushF()
	}
	return wrtn, cw.cnt, err
}

func (cw *cWrtier) flush() {
	if cw.flushWriter() {
		cw.onFlushF()
	}
}

func (cw *cWrtier) flushWriter() bool {
	cw.lock.Lock()

	if cw.w != nil {
		cw.w.flush()
	}
	if cw.iw != nil {
		cw.iw.flush()
	}

	res := cw.cntCfrmd != cw.cnt
	atomic.StoreUint32(&cw.cntCfrmd, cw.cnt)
	atomic.StoreInt64(&cw.sizeCfrmd, cw.size)
	cw.lock.Unlock()
	return res
}

func (cw *cWrtier) closeFWritersUnsafe() error {
	var err error
	if cw.w != nil || cw.iw != nil {
		if cw.w != nil {
			err = cw.w.Close()
			cw.w = nil
		}

		if cw.iw != nil {
			err1 := cw.iw.Close()
			cw.iw = nil
			if err1 != nil {
				err = err1
			}
		}

		fl := cw.cntCfrmd != cw.cnt
		atomic.StoreUint32(&cw.cntCfrmd, cw.cnt)

		close(cw.wSgnlChan)
		cw.wSgnlChan = nil

		if fl {
			go cw.onFlushF()
		}
	}
	return err
}

func (cw *cWrtier) Close() (err error) {
	atomic.StoreInt32(&cw.closed, 1)
	cw.lock.Lock()
	defer cw.lock.Unlock()

	cw.logger.Debug("Closing...")
	return cw.closeFWritersUnsafe()
}

func (cw *cWrtier) closeUnsafe() (err error) {
	atomic.StoreInt32(&cw.closed, 1)
	return cw.closeFWritersUnsafe()
}

func (cw *cWrtier) String() string {
	return fmt.Sprintf("{cntCfrmd=%d, cnt=%d, size=%d, closed=%d, idleTO=%s, flushTO=%s, maxSize=%d}",
		atomic.LoadUint32(&cw.cntCfrmd), atomic.LoadUint32(&cw.cnt), atomic.LoadInt64(&cw.size), cw.closed, cw.idleTO, cw.flushTO, cw.maxSize)
}
