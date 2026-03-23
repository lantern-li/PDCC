package blockstm

import (
	"sync"

	"github.com/emirpasic/gods/maps/treemap"
)

type (
	TxnIndex    int
	Incarnation uint
)

type TxnVersion struct {
	Index       TxnIndex
	Incarnation Incarnation
}

var InvalidTxnVersion = TxnVersion{-1, 0} // ⊥

func (v TxnVersion) Valid() bool {
	return v.Index >= 0
}

// 每笔交易的依赖
type TxDependency struct {
	sync.Mutex
	dependents []TxnIndex
}

type TaskKind int

const (
	TaskKindExecution TaskKind = iota
	TaskKindValidation
)

type Key string // comment：可以等价理解为论文中的location

type ReadDescriptor struct {
	Key Key
	// invalid Version means the key is read from storage
	Version TxnVersion
}

// txn的读集
type ReadSet struct {
	Reads []ReadDescriptor
}

// txn的写集
type WriteSet struct {
	writes []Key
}

/*
MVMemory.data 数据结构层级：
sync.Map[location] → *txnPairs
                       └── treemap[txn_idx] → *pair
                                               ├── estimate (true/false)
                                               ├── incarnation
                                               └── value
*/

type txnPairs struct {
	rw sync.RWMutex
	tm *treemap.Map // map[txn_idx]*pair
}

type pair struct {
	estimate    bool
	incarnation Incarnation
	value       []byte
}

// write-set resulting from the execution of the version
type write_set struct {
	location Key
	value    []byte
}

type ReadStatus int

type ReadResult struct {
	Status         ReadStatus
	Version        TxnVersion // ReadStatusOK 时有效
	Value          []byte     // ReadStatusOK 时有效
	BlockingTxnIdx TxnIndex   // ReadStatusError 时有效
}
