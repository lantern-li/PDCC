package blockstm

import "sync"

type (
	TxnIndex    int
	Incarnation uint
)

type TxnVersion struct {
	Index       TxnIndex
	Incarnation Incarnation
}

var InvalidTxnVersion = TxnVersion{-1, 0} // todo:这个怎么用的？

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
