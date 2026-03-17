package blockstm

import (
	"runtime"
	"sync/atomic"
)

/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

type Scheduler struct {
	block_size int

	// 有序待执行任务集合E
	execution_idx atomic.Uint64
	// 有序待验证任务集合V
	validation_idx atomic.Uint64

	// comment：以下三个用于checkout done
	// Number of times validation_idx or execution_idx was decreased
	decrease_cnt atomic.Uint64
	// Number of ongoing validation and execution tasks
	num_active_tasks atomic.Uint64
	// Marker for completion
	done_marker atomic.Bool

	// txn_idx to a mutex-protected set of dependent transaction indices
	txn_dependency []TxDependency
	// txn_idx to a mutex-protected pair (incarnation_number, status), where status ∈ {READY_TO_EXECUTE, EXECUTING, EXECUTED, ABORTING}.
	txn_status []StatusEntry

	executedTxns  atomic.Int64
	validatedTxns atomic.Int64
}

func NewScheduler(block_size int) *Scheduler {
	return &Scheduler{
		block_size:     block_size,
		txn_dependency: make([]TxDependency, block_size),
		txn_status:     make([]StatusEntry, block_size),
		// comment：atomic.Uint64不显式初始化，默认值就是 0
	}
}

// NextTask returns the transaction index and task kind for the next task to execute or validate,
// returns invalid version if no task is available.
//
// `num_active_tasks`: increased if a valid task is returned.
func (s *Scheduler) NextTask() (TxnVersion, TaskKind) {
	validation_idx := s.validation_idx.Load()
	execution_idx := s.execution_idx.Load()
	if validation_idx < execution_idx {
		return s.NextVersionToValidate(), TaskKindValidation
	} else {
		return s.NextVersionToExecute(), TaskKindExecution // comment：validation_idx = execution_idx 的情况走这个分支  一开始都取执行任务
	}
}

// NextVersionToExecute get the next transaction index to execute,
// returns invalid version if no task is available
// `num_active_tasks`: increased if a valid task is returned.
func (s *Scheduler) NextVersionToExecute() TxnVersion {
	if s.execution_idx.Load() >= uint64(s.block_size) {
		s.CheckDone()
		return InvalidTxnVersion
	}
	IncrAtomic(&s.num_active_tasks)
	idx_to_execute := s.execution_idx.Add(1) - 1
	return s.TryIncarnate(TxnIndex(idx_to_execute))
}

// NextVersionToValidate get the next transaction index to validate,
// returns invalid version if no task is available.
//
// Invariant `num_active_tasks`: increased if a valid task is returned.
func (s *Scheduler) NextVersionToValidate() TxnVersion {
	if s.validation_idx.Load() >= uint64(s.block_size) {
		s.CheckDone()
		return InvalidTxnVersion
	}
	IncrAtomic(&s.num_active_tasks)
	idx_to_validate := s.validation_idx.Add(1) - 1
	if idx_to_validate < uint64(s.block_size) {
		if ok, incarnation := s.txn_status[idx_to_validate].IsExecuted(); ok {
			return TxnVersion{TxnIndex(idx_to_validate), incarnation}
		}
	}

	DecrAtomic(&s.num_active_tasks)
	return InvalidTxnVersion
}

// TryIncarnate tries to incarnate a transaction index to execute.
// Returns the transaction version if successful, otherwise returns invalid version.
//
// `num_active_tasks`: decreased if an invalid task is returned.
func (s *Scheduler) TryIncarnate(idx TxnIndex) TxnVersion {
	if int(idx) < s.block_size {
		if incarnation, ok := s.txn_status[idx].TrySetExecuting(); ok {
			return TxnVersion{idx, incarnation}
		}
	}
	DecrAtomic(&s.num_active_tasks)
	return InvalidTxnVersion
}

func (s *Scheduler) CheckDone() {
	observed_cnt := s.decrease_cnt.Load() // todo :这个不懂，先不管这个
	if s.execution_idx.Load() >= uint64(s.block_size) &&
		s.validation_idx.Load() >= uint64(s.block_size) &&
		s.num_active_tasks.Load() == 0 {
		if observed_cnt == s.decrease_cnt.Load() {
			s.done_marker.Store(true)
		}
	}
	// avoid busy waiting
	runtime.Gosched()
}

func (s *Scheduler) Done() bool {
	return s.done_marker.Load()
}
