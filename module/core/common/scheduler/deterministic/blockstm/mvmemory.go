package blockstm

import (
	"sync"
	"sync/atomic"

	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/utils"
)

const (
	ReadStatusOK       ReadStatus = iota
	ReadStatusNotFound            // S = {}，location 在 MVdata 中无任何 idx < txn_idx 的条目
	ReadStatusError               // entry = ESTIMATE，需等待 blocking_txn_idx 完成
)

// blockstm中的MVMemory
type MVMemory struct {
	data sync.Map

	// 读集和写集也要是原子更新的
	lastWrittenLocations []atomic.Pointer[WriteSet]
	lastReadSet          []atomic.Pointer[ReadSet]

	// 记录每笔交易最后一次incarnation执行的txSimContext
	lastTxSimContext []atomic.Pointer[protocol.TxSimContext]
}

func NewMVMemory(blockSize int) *MVMemory {
	return &MVMemory{
		lastWrittenLocations: make([]atomic.Pointer[WriteSet], blockSize),
		lastReadSet:          make([]atomic.Pointer[ReadSet], blockSize),
		lastTxSimContext:     make([]atomic.Pointer[protocol.TxSimContext], blockSize),
	}
}

func (mv *MVMemory) apply_write_set(txn_index TxnIndex, incarnation_number Incarnation, write_set []write_set) {
	for _, write := range write_set {
		newPair := &pair{
			estimate:    false,
			incarnation: incarnation_number,
			value:       write.value,
		}

		actual, _ := mv.data.LoadOrStore(write.location, &txnPairs{
			tm: treemap.NewWith(utils.IntComparator),
		})
		tp := actual.(*txnPairs)
		tp.rw.Lock()
		tp.tm.Put(int(txn_index), newPair)
		tp.rw.Unlock()
	}
}

/*
function rcu_update_written_locations(txn_index, new_locations)

	prev_locations ← last_written_locations[txn_idx]        ⊲ loaded atomically (RCU read)
	for every unwritten_location ∈ prev_locations \ new_locations do
		data.remove((unwritten_location, txn_idx))          ⊲ remove entries that were not overwritten
	last_written_locations[txn_idx] ← new_locations         ⊲ store newly written locations atomically (RCU update)

return new_locations \ prev_locations ≠ {} 					⊲ was there a write to a location not written the last time
*/
func (mv *MVMemory) rcu_update_written_locations(txn_index TxnIndex, new_locations map[Key]struct{}) bool {
	// prev_locations ← last_written_locations[txn_idx]  (RCU read)
	prev := mv.lastWrittenLocations[txn_index].Load()

	// for every unwritten_location ∈ prev_locations \ new_locations do
	//     data.remove((unwritten_location, txn_idx))
	if prev != nil {
		for _, loc := range prev.writes {
			if _, exists := new_locations[loc]; !exists {
				if val, ok := mv.data.Load(loc); ok { // todo：这里应该肯定存在，若不存在应该报个错
					tp := val.(*txnPairs)
					tp.rw.Lock()
					tp.tm.Remove(int(txn_index))
					tp.rw.Unlock()
				}
			}
		}
	}

	// last_written_locations[txn_idx] ← new_locations  (RCU update)
	ws := &WriteSet{writes: make([]Key, 0, len(new_locations))}
	for loc := range new_locations {
		ws.writes = append(ws.writes, loc) // todo这里的写无序，但不影响
	}
	mv.lastWrittenLocations[txn_index].Store(ws) // todo考虑是否需要CAS

	// return new_locations \ prev_locations ≠ {}
	if prev == nil {
		return len(new_locations) > 0
	}
	prevSet := make(map[Key]struct{}, len(prev.writes))
	for _, k := range prev.writes {
		prevSet[k] = struct{}{}
	}
	for loc := range new_locations {
		if _, exists := prevSet[loc]; !exists {
			return true // 只要有一个就直接返回true todo确认
		}
	}
	return false
}

func (mv *MVMemory) record(version TxnVersion, read_set ReadSet, write_set []write_set, txSimContext protocol.TxSimContext) bool {
	txn_idx := version.Index
	incarnation_number := version.Incarnation
	mv.apply_write_set(txn_idx, incarnation_number, write_set)

	// new_locations ← {location | (location,★) ∈ write_set}
	new_locations := make(map[Key]struct{}, len(write_set))
	for _, w := range write_set {
		new_locations[w.location] = struct{}{}
	}

	//wrote_new_location ← rcu_update_written_locations(txn_idx, new_locations)
	wrote_new_location := mv.rcu_update_written_locations(txn_idx, new_locations)

	// last_read_set[txn_idx] ← read_set ⊲ store the read-set atomically (RCU update)
	mv.lastReadSet[txn_idx].Store(&read_set)

	// 记录最后一次incarnation执行的txSimContext
	mv.lastTxSimContext[txn_idx].Store(&txSimContext)
	return wrote_new_location
}

// procedure convert_writes_to_estimates(txn_idx)
//
//	prev_locations ← last_written_locations[txn_idx] ⊲ loaded atomically (RCU read)
//	for every location ∈ prev_location do
//		data[(location, txn_idx)] ← ESTIMATE ⊲ entry is guaranteed to exist
func (mv *MVMemory) convert_writes_to_estimates(txn_idx TxnIndex) {
	// prev_locations ← last_written_locations[txn_idx]  (RCU read)
	prev := mv.lastWrittenLocations[txn_idx].Load()
	if prev == nil {
		return
	}
	// for every location ∈ prev_locations do
	//     data[(location, txn_idx)] ← ESTIMATE
	for _, loc := range prev.writes {
		if val, ok := mv.data.Load(loc); ok { // 同样这里肯定是可以加载出来的
			tp := val.(*txnPairs)
			tp.rw.Lock()
			if entry, found := tp.tm.Get(int(txn_idx)); found { // 肯定
				entry.(*pair).estimate = true
			}
			tp.rw.Unlock()
		}
	}
}

// function read(location, txn_idx)
//
//	𝑆 ← {((location, idx), entry) ∈ data | idx < txn_idx}
//	if 𝑆 = {} then
//		return (status ← NOT_FOUND)
//	((location, idx), entry) ← arg max𝑖𝑑𝑥 S
//	if entry = ESTIMATE then
//		return (status ← READ_ERROR, blocking_txn_idx ← idx)
//	return (status←OK, version←(idx, entry.incarnation_number),value ← entry.value)
func (mv *MVMemory) read(location Key, txn_idx TxnIndex) ReadResult {
	// S ← {((location, idx), entry) ∈ data | idx < txn_idx}
	// arg max_idx S：treemap 有序，Floor(txn_idx-1) 即最大的 idx < txn_idx
	val, ok := mv.data.Load(location)
	if !ok {
		return ReadResult{Status: ReadStatusNotFound} // 这里好像该填充无效版本，或后面去掉vaild
	}
	tp := val.(*txnPairs)
	tp.rw.RLock()
	foundKey, foundVal := tp.tm.Floor(int(txn_idx) - 1) // Floor(x) = 返回 ≤ x 的最大 key
	tp.rw.RUnlock()

	// if S = {} then return NOT_FOUND
	if foundKey == nil {
		return ReadResult{Status: ReadStatusNotFound}
	}

	idx := TxnIndex(foundKey.(int))
	entry := foundVal.(*pair)

	// if entry = ESTIMATE then return READ_ERROR
	if entry.estimate {
		return ReadResult{Status: ReadStatusError, BlockingTxnIdx: idx}
	}

	// return OK
	return ReadResult{
		Status:  ReadStatusOK,
		Version: TxnVersion{Index: idx, Incarnation: entry.incarnation},
		Value:   entry.value,
	}
}

/*
function validate_read_set(txn_idx)

	prior_reads ← last_read_set[txn_idx] 						⊲ last recorded read_set, loaded atomically via RCU
	for every (location, version) ∈ prior_reads do 				⊲ version is ⊥ when prior read returned NOT_FOUND
		cur_read ← read(location, txn_idx)
		if cur_read.status = READ_ERROR then
			return false 										⊲ previously read entry from data, now ESTIMATE
		if cur_read.status = NOT_FOUND ∧ version ≠ ⊥ then
			return false 										⊲ previously read entry from data, now NOT_FOUND
		if cur_read.status = OK ∧ cur_read.version ≠ version then
		return false 											⊲ read some entry, but not the same as before
	return true
*/
func (mv *MVMemory) validate_read_set(txn_idx TxnIndex) bool {
	// prior_reads ← last_read_set[txn_idx]  (RCU read)
	prior := mv.lastReadSet[txn_idx].Load()
	if prior == nil {
		return true
	}
	for _, rd := range prior.Reads {
		// cur_read ← read(location, txn_idx)
		curRead := mv.read(rd.Key, txn_idx)

		if curRead.Status == ReadStatusError {
			return false
		}
		// version ≠ ⊥ 即上次读到了 MVdata 中的某个版本
		if curRead.Status == ReadStatusNotFound && rd.Version.Valid() { // todo：思考这个rd.Version.Valid()，是否需要去掉
			return false
		}
		if curRead.Status == ReadStatusOK && curRead.Version != rd.Version {
			return false
		}
	}
	return true
}

/*
function snapshot()
	ret ← {}
	for every location | ((location, ★), ★) ∈ data do
	result ← read(location, BLOCK.size())
	if result.status = OK then
		ret ← ret ∪ {location, result.value}
	return ret
*/ // todo：blockstm的这个接口在chainmaker这里用不上，blockstm和chainmaker这里的落库方式不同
func (mv *MVMemory) snapshot() map[Key][]byte {
	blockSize := TxnIndex(len(mv.lastWrittenLocations))
	ret := make(map[Key][]byte)
	mv.data.Range(func(k, _ any) bool {
		location := k.(Key)
		result := mv.read(location, blockSize)
		if result.Status == ReadStatusOK {
			ret[location] = result.Value
		}
		return true
	})
	return ret
}
