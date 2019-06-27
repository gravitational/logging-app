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

package tmindex

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jrivets/log4g"
	"github.com/logrange/logrange/pkg/model"
	"github.com/logrange/range/pkg/records/chunk"
	sync2 "github.com/logrange/range/pkg/sync"
	errors2 "github.com/logrange/range/pkg/utils/errors"
	"github.com/logrange/range/pkg/utils/fileutil"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"math"
	"path"
	"sync"
	"time"
)

type (
	// cindex struct allows to keep information about known chunks, this
	// naive implementation keeps everything in memory (but not ts index per chunk which is
	// stored in ckindex).
	cindex struct {
		lock sync.Mutex

		// journals build association between known partitions and its chunks
		journals   map[string]sortedChunks
		logger     log4g.Logger
		dtFileName string

		cc *ckiCtrlr
	}

	// sortedChunks slice contains information about a partition's chunks ordered by their chunkId
	sortedChunks []*chkInfo

	// chkInfo struct keeps information about a chunk. Used by cindex
	chkInfo struct {
		Id    chunk.Id
		MinTs int64
		MaxTs int64

		// the rwLock is used to access to the ckIndex and it guards
		// the following fields - IdxRoot, lastRec, idxCorrupted.
		// the values could be changed only when Write lock on rwLock is acquired
		rwLock  sync2.RWLock
		IdxRoot Item
		lastRec uint32
		// returns whether the ckIndex is considered
		idxCorrupted bool
	}
)

// cIndexFileName contains the name of file where cindex information will be persisted
const cIndexFileName = "cindex.dat"

// sparseSpace defines a coefficient how often to fix timestamp point in the index
const sparseSpace = 250

func newCIndex() *cindex {
	ci := new(cindex)
	ci.logger = log4g.GetLogger("cindex")
	ci.journals = make(map[string]sortedChunks)
	return ci
}

func (ci *cindex) init(ddir string) error {
	ci.logger.Info("Init()")
	err := fileutil.EnsureDirExists(ddir)
	if err != nil {
		ci.logger.Error("cindex.init(): could not create dir ", ddir, ", err=", err)
		return err
	}

	ci.dtFileName = path.Join(ddir, cIndexFileName)
	ci.logger.Info("Set data file to ", ci.dtFileName)
	ci.loadDataFromFile()

	ci.cc = newCkiCtrlr()
	err = ci.cc.init(ddir, 50)
	if err != nil {
		return err
	}

	aiMap := make(map[uint64]bool)
	for _, sc := range ci.journals {
		for _, c := range sc {
			if c.IdxRoot.IndexId != 0 {
				aiMap[c.IdxRoot.IndexId] = true
			}
		}
	}
	ci.cc.cleanup(aiMap)

	return nil
}

func (ci *cindex) close() error {
	err := ci.cc.close()
	ci.saveDataToFile()
	return err
}

// onWrite updates the partition cindex. It receives the partition name src, number of records in the
// chunk and the rInfo
func (ci *cindex) onWrite(src string, firstRec, lastRec uint32, rInfo RecordsInfo) (err error) {
	ci.lock.Lock()
	newChk := false
	sc, ok := ci.journals[src]
	if !ok {
		sc = make(sortedChunks, 1)
		ci.journals[src] = sc
		sc[0] = &chkInfo{Id: rInfo.Id, MaxTs: rInfo.MaxTs, MinTs: rInfo.MinTs}
		newChk = true
	}

	if sc[len(sc)-1].Id != rInfo.Id {
		// seems like we have a new chunk
		sc = append(sc, &chkInfo{Id: rInfo.Id, MaxTs: rInfo.MaxTs, MinTs: rInfo.MinTs})
		ci.journals[src] = sc
		newChk = true
	} else if ok {
		sc[len(sc)-1].update(rInfo)
	}

	last := sc[len(sc)-1]
	ci.lock.Unlock()

	// now check whether we have to call for the index update
	if !last.rwLock.TryLock() {
		return nil
	}

	if last.idxCorrupted {
		last.rwLock.Unlock()
		return ErrTmIndexCorrupted
	}

	// if the last chunk has been just created, but the notification is not first for the chunk,
	// declare it as corrupted (make the call to rebuild it)
	if newChk && firstRec > 0 {
		last.makeCorrupted()
		last.rwLock.Unlock()
		return ErrTmIndexCorrupted
	}

	if last.lastRec > 0 && lastRec-last.lastRec < sparseSpace {
		// no need to write, give it a space so far
		last.rwLock.Unlock()
		return nil
	}

	it := interval{record{rInfo.MinTs, firstRec}, record{rInfo.MaxTs, lastRec}}
	if last.IdxRoot.IndexId == 0 {
		if lastRec-last.lastRec > sparseSpace*20 {
			// well, it is big gap in the index, let's make it rebuilt later...
			last.makeCorrupted()
			last.rwLock.Unlock()
			return ErrTmIndexCorrupted
		}

		root, err := ci.cc.arrangeRoot(it)
		if err != nil {
			ci.logger.Warn("could not create root element for ", rInfo, ", err=", err)
			last.makeCorrupted()
			last.rwLock.Unlock()
			return ErrTmIndexCorrupted
		}

		last.lastRec = lastRec
		last.IdxRoot = root
		last.rwLock.Unlock()
		return nil
	}

	last.IdxRoot, err = ci.cc.onWrite(last.IdxRoot, it)
	if err != nil {
		ci.logger.Warn("could not add new record to the index, err=", err, " last=", last)
		last.IdxRoot = Item{}
		last.idxCorrupted = true
	}
	last.lastRec = lastRec
	last.rwLock.Unlock()

	return err
}

func (ci *cindex) getPosForGreaterOrEqualTime(src string, cid chunk.Id, ts int64) (uint32, error) {
	ci.lock.Lock()
	sc, ok := ci.journals[src]
	if !ok {
		ci.lock.Unlock()
		return 0, errors2.NotFound
	}

	idx := sc.findChunkIdx(cid)
	if idx < 0 {
		ci.lock.Unlock()
		return 0, errors2.NotFound
	}

	cinfo := sc[idx]
	if cinfo.MaxTs < ts {
		ci.lock.Unlock()
		return 0, ErrOutOfRange
	}

	if cinfo.MinTs >= ts {
		ci.lock.Unlock()
		return 0, nil
	}

	ci.lock.Unlock()

	cinfo.rwLock.RLock()
	if !cinfo.idxCorrupted {
		pos, err := ci.cc.grEq(cinfo.IdxRoot, int64(ts))
		if err == errAllMatches {
			pos = 0
			err = nil
		}

		if err == nil {
			cinfo.rwLock.RUnlock()
			return pos, nil
		}
	}
	cinfo.rwLock.RUnlock()

	return 0, ErrTmIndexCorrupted
}

func (ci *cindex) getPosForLessTime(src string, cid chunk.Id, ts int64) (uint32, error) {
	ci.lock.Lock()
	sc, ok := ci.journals[src]
	if !ok {
		ci.lock.Unlock()
		return 0, errors2.NotFound
	}

	idx := sc.findChunkIdx(cid)
	if idx < 0 {
		ci.lock.Unlock()
		return 0, errors2.NotFound
	}

	cinfo := sc[idx]
	if cinfo.MaxTs <= ts {
		ci.lock.Unlock()
		return math.MaxUint32, nil
	}

	if cinfo.MinTs >= ts {
		ci.lock.Unlock()
		return 0, ErrOutOfRange
	}

	ci.lock.Unlock()

	cinfo.rwLock.RLock()
	if !cinfo.idxCorrupted {
		pos, err := ci.cc.less(cinfo.IdxRoot, int64(ts))
		cinfo.rwLock.RUnlock()
		if err == nil {
			return pos, nil
		}
		// whell, in case the error, we are still not sure where to start, so just start from the end
		return math.MaxUint32, nil
	}
	cinfo.rwLock.RUnlock()

	return 0, ErrTmIndexCorrupted

}

// lastChunkRecordsInfo returns the last chunk information. Returns NotFound if there is
// no such source
func (ci *cindex) lastChunkRecordsInfo(src string) (res RecordsInfo, err error) {
	err = errors2.NotFound
	ci.lock.Lock()
	if sc, ok := ci.journals[src]; ok && len(sc) > 0 {
		res = sc[len(sc)-1].getRecordsInfo()
		err = nil
	}
	ci.lock.Unlock()
	return
}

func (ci *cindex) getRecordsInfo(src string, cid chunk.Id) (res RecordsInfo, err error) {
	err = errors2.NotFound
	ci.lock.Lock()
	if sc, ok := ci.journals[src]; ok && len(sc) > 0 {
		cidx := sc.findChunkIdx(cid)
		if cidx >= 0 {
			res = sc[cidx].getRecordsInfo()
			err = nil
		}
	}
	ci.lock.Unlock()
	return
}

func (ci *cindex) count(src string, cid chunk.Id) (count int, err error) {
	err = errors2.NotFound
	var cinfo *chkInfo
	ci.lock.Lock()
	if sc, ok := ci.journals[src]; ok && len(sc) > 0 {
		cidx := sc.findChunkIdx(cid)
		if cidx >= 0 {
			cinfo = sc[cidx]
			err = nil
		}
	}
	ci.lock.Unlock()

	if cinfo == nil {
		return
	}

	cinfo.rwLock.RLock()
	if cinfo.IdxRoot.IndexId == 0 || cinfo.idxCorrupted {
		cinfo.rwLock.RUnlock()
		return 0, ErrTmIndexCorrupted
	}

	cki := ci.cc.getIndex(cinfo.IdxRoot.IndexId)
	if cki == nil {
		cinfo.rwLock.RUnlock()
		return 0, ErrTmIndexCorrupted
	}

	count, err = cki.count(cinfo.IdxRoot.Pos)

	cinfo.rwLock.RUnlock()

	return
}

// rebuildIndex runs the building new index for the source and the chunk provided
func (ci *cindex) rebuildIndex(ctx context.Context, src string, chk chunk.Chunk, force bool) {
	ci.lock.Lock()
	var res *chkInfo
	if sc, ok := ci.journals[src]; ok {
		idx := sc.findChunkIdx(chk.Id())
		if idx >= 0 {
			res = sc[idx]
		}
	}
	ci.lock.Unlock()

	if res == nil {
		ci.logger.Error("rebuildIndex(): no partiton ", src, ", or such chunk there")
		return
	}

	if err := res.rwLock.LockWithCtx(ctx); err != nil {
		ci.logger.Error("rebuildIndex(): could not acquire write lock, err=", err)
		return
	}

	if res.IdxRoot.IndexId != 0 && !res.idxCorrupted {
		if ci.cc.getIndex(res.IdxRoot.IndexId) != nil {
			if !force {
				ci.logger.Warn("The index for chunk ", chk, " for partition \"", src, "\" seems to be ok and alive. Do not build one. ")
				res.rwLock.Unlock()
				return
			}
			ci.cc.removeIndex(res.IdxRoot.IndexId)
		}
		res.makeCorrupted()
	}

	rInfo, root, err := ci.rebuildIndexInt(ctx, chk)
	if err != nil {
		res.rwLock.Unlock()
		return
	}

	res.IdxRoot = root
	res.idxCorrupted = false
	res.lastRec = 0
	res.rwLock.Unlock()

	// To change the rInfo acquire another lock
	ci.lock.Lock()
	found := false
	if sc, ok := ci.journals[src]; ok {
		for _, c := range sc {
			if c == res {
				found = true
				res.update(rInfo)
				break
			}
		}
	}
	ci.lock.Unlock()

	if !found {
		ci.logger.Warn("rebuildIndex(): chunk either disappear or partition was deleted ", chk, " partition ", src)
		ci.cc.removeItem(root)
	}
}

func (ci *cindex) writeIndexInterval(root Item, ri RecordsInfo, pos0, pos1 int) (Item, error) {
	if pos0 == pos1 {
		return root, nil
	}

	var err error
	intv := interval{record{ri.MinTs, uint32(pos0)}, record{ri.MaxTs, uint32(pos1)}}
	root, err = ci.cc.onWrite(root, intv)
	if err != nil {
		ci.cc.removeItem(root)
		ci.logger.Error("rebuildIndexInt(): could not write index info for ", pos0, " .. ", pos1, ", err=", err)
	}
	return root, err
}

// rebuildIndexInt allows to check the chunk's records from the chunk remembering
// their time points and positions in the time index.
func (ci *cindex) rebuildIndexInt(ctx context.Context, chk chunk.Chunk) (RecordsInfo, Item, error) {
	var rInfo, segmInfo RecordsInfo
	var root Item
	var err error

	rInfo.Id = chk.Id()

	if chk.Count() > 0 {
		var it chunk.Iterator
		it, err = chk.Iterator()
		if err != nil {
			ci.logger.Error("rebuildIndexInt(): could not create iterator, err=", err)
			return rInfo, root, err
		}
		defer it.Close()

		ts, err := getRecordTimestamp(ctx, it)
		if err != nil {
			ci.logger.Error("rebuildIndexInt(): could not read first record, err=", err)
			return rInfo, root, err
		}

		intv := interval{record{ts, uint32(0)}, record{ts, uint32(0)}}
		root, err = ci.cc.arrangeRoot(intv)
		if err != nil {
			ci.logger.Warn("rebuildIndexInt(): could not create root element for ", rInfo, ", err=", err)
			return rInfo, root, err
		}
		rInfo.MaxTs = ts
		rInfo.MinTs = ts
		segmInfo = RecordsInfo{MinTs: math.MaxInt64}

		pos0 := 0
		pos1 := 0
		for err == nil {
			ts, err = getRecordTimestamp(ctx, it)
			if err == io.EOF {
				root, err = ci.writeIndexInterval(root, segmInfo, pos0, pos1)
				break
			}

			if err != nil {
				ci.cc.removeItem(root)
				ci.logger.Error("rebuildIndexInt(): could not read next record, err=", err)
				return rInfo, root, err
			}
			it.Next(ctx)

			rInfo.ApplyTs(ts)
			segmInfo.ApplyTs(ts)
			pos1++
			if pos1-pos0 < sparseSpace {
				continue
			}

			root, err = ci.writeIndexInterval(root, segmInfo, pos0, pos1)
			pos0 = pos1
			segmInfo = RecordsInfo{MinTs: math.MaxInt64}
		}
	} else {
		ci.logger.Info("rebuildIndex(): the chunk ", chk, " has 0 size")
	}

	return rInfo, root, err
}

// readData reads all index data and returns it as a slice []IdxRecord
func (ci *cindex) readData(src string, cid chunk.Id) ([]IdxRecord, error) {
	ci.lock.Lock()
	var res *chkInfo
	if sc, ok := ci.journals[src]; ok {
		idx := sc.findChunkIdx(cid)
		if idx >= 0 {
			res = sc[idx]
		}
	}
	ci.lock.Unlock()

	if res == nil {
		ci.logger.Debug("readData(): no recods for partition \"", src, "\" cid=", cid)
		return nil, errors2.NotFound
	}

	res.rwLock.RLock()

	if res.idxCorrupted {
		ci.logger.Debug("readData(): could not read data for partition \"", src, "\" cid=", cid, " index corrupted.")
		res.rwLock.RUnlock()
		return nil, ErrTmIndexCorrupted
	}

	cki := ci.cc.getIndex(res.IdxRoot.IndexId)
	if cki == nil {
		ci.logger.Debug("readData(): could not read data for partition \"", src, "\" cid=", cid, " index not found by IndexId")
		res.rwLock.RUnlock()
		return nil, errors2.NotFound
	}

	cnt, err := cki.count(res.IdxRoot.Pos)
	if err != nil {
		ci.logger.Warn("readData(): could not count records in index for partition \"", src, "\" cid=", cid, ", err=", err)
		res.rwLock.RUnlock()
		return nil, err
	}

	rr := make([]interval, 0, cnt)
	rr, err = cki.traversal(res.IdxRoot.Pos, rr)
	if err != nil {
		ci.logger.Warn("readData(): could not traverse index for partition \"", src, "\" cid=", cid, ", err=", err)
		res.rwLock.RUnlock()
		return nil, err
	}

	ir := make([]IdxRecord, len(rr)+1)
	for i, it := range rr {
		ir[i] = IdxRecord{it.p0.ts, it.p0.idx}
	}

	lastIdx := len(rr)
	if lastIdx > 0 {
		it := rr[lastIdx-1]
		ir[lastIdx] = IdxRecord{it.p1.ts, it.p1.idx}
	}

	res.rwLock.RUnlock()
	return ir, nil
}

// syncChunks receives a list of chunks for a partition src and updates the cindex information by the data from the chunks
func (ci *cindex) syncChunks(ctx context.Context, src string, cks chunk.Chunks) []RecordsInfo {
	newSC := chunksToSortedChunks(cks)

	ci.lock.Lock()
	if sc, ok := ci.journals[src]; ok {
		// re-assignment cause apply can re-allocate original slice
		newSC, _ = newSC.apply(sc, true)
	}
	ci.lock.Unlock()

	ci.lightFill(ctx, cks, newSC)

	ci.lock.Lock()
	sc := ci.journals[src]
	newSC, rmvd := newSC.apply(sc, false)
	if len(newSC) > 0 {
		ci.journals[src] = newSC
	} else {
		delete(ci.journals, src)
	}
	res := newSC.makeRecordsInfoCopy()
	ci.lock.Unlock()

	ci.dropSortedChunks(ctx, rmvd)
	return res
}

func (ci *cindex) visitSources(visitor func(src string) bool) {
	ci.lock.Lock()
	srcs := make([]string, len(ci.journals))
	i := 0
	for s := range ci.journals {
		srcs[i] = s
		i++
	}
	ci.lock.Unlock()

	ci.logger.Debug("VisitSources(): will visit ", len(srcs), " sources")
	for i, src := range srcs {
		if !visitor(src) {
			ci.logger.Debug("VisitSources(): interrupted by vistior at idx=", i)
			return
		}
	}
}

func (ci *cindex) dropSortedChunks(ctx context.Context, rmvd sortedChunks) {
	for _, r := range rmvd {
		if err := r.rwLock.LockWithCtx(ctx); err != nil {
			ci.logger.Error("Could not obtain read lock. potential leak!!!")
			go ci.cc.removeIndex(r.IdxRoot.IndexId)
			continue
		}

		if r.IdxRoot.IndexId != 0 {
			ci.logger.Debug("Remove tm index for chunk ", r.Id)
			ci.cc.removeItem(r.IdxRoot)
		}
		r.rwLock.Unlock()
	}
}

func (ci *cindex) lightFill(ctx context.Context, cks chunk.Chunks, sc sortedChunks) {
	j := 0
	for _, c := range sc {
		if c.MaxTs > 0 {
			continue
		}

		for ; j < len(cks) && cks[j].Id() < c.Id; j++ {
		}

		if j == len(cks) {
			return
		}

		chk := cks[j]
		if chk.Id() != c.Id {
			panic(fmt.Sprintf("lightFill(): could not find chunk for id=%v check the code!", c.Id))
		}

		if chk.Count() == 0 {
			ci.logger.Debug("lightFill(): empty chunk, continue.")
			continue
		}

		it, err := chk.Iterator()
		if err != nil {
			ci.logger.Error("lightFill(): Could not create iterator, err=", err)
			continue
		}

		ts1, err := getRecordTimestamp(ctx, it)
		if err != nil {
			it.Close()
			ci.logger.Warn("lightFill(): Could not read first record, err=", err)
			continue
		}

		it.SetPos(int64(chk.Count()) - 1)
		ts2, err := getRecordTimestamp(ctx, it)
		if err != nil {
			it.Close()
			ci.logger.Warn("lightFill(): Could not read last record, err=", err)
			continue
		}

		c.MinTs = ts1
		c.MaxTs = ts2
		if ts2 < ts1 {
			ci.logger.Warn("lightFill(): first record of chunk ", chk.Id(), " has greater timestamp, than its last one")
			c.MinTs = ts2
			c.MaxTs = ts1
		}
		it.Close()
	}
}

func getRecordTimestamp(ctx context.Context, it chunk.Iterator) (int64, error) {
	rec, err := it.Get(ctx)
	if err != nil {
		return 0, err
	}

	var le model.LogEvent
	_, err = le.Unmarshal(rec, false)
	if err != nil {
		return 0, errors.Wrapf(err, "getRecordTimestamp(): could not unmarshal record")
	}
	return le.Timestamp, nil
}

func (ci *cindex) loadDataFromFile() {
	data, err := ioutil.ReadFile(ci.dtFileName)
	if err != nil {
		ci.logger.Warn("loadDataFromFile(): could not read data from file, the err=", err)
		return
	}

	err = json.Unmarshal(data, &ci.journals)
	if err != nil {
		ci.logger.Warn("loadDataFromFile(): could not unmarshal data. err=", err)
		return
	}
	ci.logger.Info("successfully read information about ", len(ci.journals), " journals from ", ci.dtFileName)
}

func (ci *cindex) saveDataToFile() {
	if len(ci.dtFileName) == 0 {
		ci.logger.Warn("Could not persist data, no file name.")
		return
	}

	data, err := json.Marshal(ci.journals)
	if err != nil {
		ci.logger.Error("Could not persist data to file ", ci.dtFileName, ", err=", err)
		return
	}

	if err = ioutil.WriteFile(ci.dtFileName, data, 0640); err != nil {
		ci.logger.Error("could not save data to ", ci.dtFileName, ", err=", err)
		return
	}

	ci.logger.Info("saved all data to ", ci.dtFileName)
}

func chunksToSortedChunks(cks chunk.Chunks) sortedChunks {
	res := make(sortedChunks, len(cks))
	for i, ck := range cks {
		res[i] = &chkInfo{Id: ck.Id(), MinTs: math.MaxInt64}
	}
	return res
}

// apply overwrites values in sc by values sc1. It returns 2 sorted slices
// the first one is the new merged chunks and the second one is sorted chunks
// that were removed from sc1.
func (sc sortedChunks) apply(sc1 sortedChunks, copyData bool) (sortedChunks, sortedChunks) {
	i := 0
	j := 0
	removed := make(sortedChunks, 0, 1)
	for i < len(sc) && j < len(sc1) {
		if sc[i].Id > sc1[j].Id {
			removed = append(removed, sc1[j])
			j++
			continue
		}
		if sc[i].Id < sc1[j].Id {
			i++
			continue
		}

		if copyData {
			sc[i].MaxTs = sc1[j].MaxTs
			sc[i].MinTs = sc1[j].MinTs
		} else {
			sc[i] = sc1[j]
		}

		i++
		j++
	}

	// add all tailed chunks that are not in sc to sc
	for ; j < len(sc1); j++ {
		removed = append(removed, sc1[j])
	}

	// returns sc cause it could re-allocate original slice
	return sc, removed
}

func (sc sortedChunks) makeRecordsInfoCopy() []RecordsInfo {
	res := make([]RecordsInfo, len(sc))
	for i, c := range sc {
		res[i] = c.getRecordsInfo()
	}
	return res
}

func (sc sortedChunks) findChunkIdx(cid chunk.Id) int {
	i, j := 0, len(sc)
	for i < j {
		h := int(uint(i+j) >> 1)
		if sc[h].Id == cid {
			return h
		}

		if sc[h].Id < cid {
			i = h + 1
		} else {
			j = h
		}
	}
	return -1
}

func (ci *chkInfo) getRecordsInfo() RecordsInfo {
	return RecordsInfo{Id: ci.Id, MinTs: ci.MinTs, MaxTs: ci.MaxTs}
}

func (ci *chkInfo) update(rInfo RecordsInfo) bool {
	if ci.MinTs > rInfo.MinTs {
		ci.MinTs = rInfo.MinTs
	}
	if ci.MaxTs < rInfo.MaxTs {
		ci.MaxTs = rInfo.MaxTs
	}
	return true
}

func (ci *chkInfo) makeCorrupted() {
	ci.idxCorrupted = true
	ci.IdxRoot = Item{}
}

func (ci *chkInfo) String() string {
	minTs := time.Unix(0, int64(ci.MinTs))
	maxTs := time.Unix(0, int64(ci.MaxTs))
	return fmt.Sprintf("chkInfo:{Id: %s, MinTs: %s, MaxTs: %s, IdxRoot: %s, lastRec: %d, corrupted: %t}", ci.Id, minTs, maxTs, ci.IdxRoot, ci.lastRec, ci.idxCorrupted)
}
