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
	decrease_cnt atomic.Uint64 // 记录了“拨回”操作发生了多少次
	// Number of ongoing validation and execution tasks
	num_active_tasks atomic.Uint64
	// Marker for completion
	done_marker atomic.Bool

	// txn_idx to a mutex-protected set of dependent transaction indices
	txn_dependency []TxDependency // 这里是阻塞者 导致——> 被阻塞者
	// txn_idx to a mutex-protected pair (incarnation_number, status), where status ∈ {READY_TO_EXECUTE, EXECUTING, EXECUTED, ABORTING}.
	txn_status []StatusEntry
}

func NewScheduler(block_size int) *Scheduler {
	return &Scheduler{
		block_size:     block_size,
		txn_dependency: make([]TxDependency, block_size),
		txn_status:     make([]StatusEntry, block_size),
		// comment：atomic.Uint64不显式初始化，默认值就是 0
	}
}

/*
procedure decrease_execution_idx(target_idx)

	execution_idx ← min(execution_idx, target_idx) ⊲ atomic
	decrease_cnt.increment()
*/
func (s *Scheduler) decrease_execution_idx(target_idx TxnIndex) {
	for {
		current := s.execution_idx.Load()
		if current <= uint64(target_idx) {
			break
		}
		if s.execution_idx.CompareAndSwap(current, uint64(target_idx)) { // todo：了解atomic.Uint64的CAS
			break
		}
	}
	s.decrease_cnt.Add(1) // 每拨回一次就加1
}

func (s *Scheduler) Done() bool {
	return s.done_marker.Load()
}

/*
procedure decrease_validation_idx(target_idx)

	validation_idx ← min(validation_idx, target_idx) ⊲ atomic
	decrease_cnt.increment()
*/
func (s *Scheduler) decrease_validation_idx(target_idx TxnIndex) {
	for {
		current := s.validation_idx.Load()
		if current <= uint64(target_idx) {
			break
		}
		if s.validation_idx.CompareAndSwap(current, uint64(target_idx)) {
			break
		}
	}
	s.decrease_cnt.Add(1) // 每拨回一次就加1
}

/*
procedure check_done()
	observed_cnt ← decrease_cnt
	if min(execution_idx, validation_idx) ≥ BLOCK.size() ∧
		num_active_tasks = 0 ∧ observed_cnt = decrease_cnt then
		done_marker ← true
*/

func (s *Scheduler) CheckDone() {
	observed_cnt := s.decrease_cnt.Load()
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

/*
function try_incarnate(txn_idx)
	if txn_idx < BLOCK.size() then
		with txn_status[txn_idx].lock()
			if txn_status[txn_idx].status = READY_TO_EXECUTE then
				txn_status[txn_idx].status ← EXECUTING
				return (txn_idx, txn_status[txn_idx].incarnation_number)
	num_active_tasks.decrement()
	return ⊥
*/
// try_incarnate tries to incarnate a transaction index to execute.
func (s *Scheduler) try_incarnate(txn_idx TxnIndex) TxnVersion {
	// if txn_idx < BLOCK.size() then
	if int(txn_idx) < s.block_size {
		// with txn_status[txn_idx].lock()
		entry := &s.txn_status[txn_idx]
		entry.Lock()
		// if txn_status[txn_idx].status = READY_TO_EXECUTE then
		if entry.status == StatusReadyToExecute {
			// txn_status[txn_idx].status ← EXECUTING
			entry.status = StatusExecuting
			incarnation := entry.incarnation
			entry.Unlock()
			// return (txn_idx, txn_status[txn_idx].incarnation_number)
			return TxnVersion{txn_idx, incarnation}
		}
		entry.Unlock()
	}
	// num_active_tasks.decrement()
	s.num_active_tasks.Add(^uint64(0)) // 减1
	// return ⊥
	return InvalidTxnVersion
}

// NextTask returns the transaction index and task kind for the next task to execute or validate,
// returns invalid version if no task is available.
//
// `num_active_tasks`: increased if a valid task is returned.
/*
function next_task()
if validation_idx < execution_idx then
version_to_validate ← next_version_to_validate()
if version_to_validate ≠ ⊥ then
return (version ← version_to_validate,kind ← VALIDATION_TASK)
else
version_to_execute ← next_version_to_execute()
if version_to_execute ≠ ⊥ then
return (version ← version_to_execute,kind ← EXECUTION_TASK)
return ⊥
*/
func (s *Scheduler) NextTask() (TxnVersion, TaskKind) {
	validation_idx := s.validation_idx.Load()
	execution_idx := s.execution_idx.Load()
	if validation_idx < execution_idx {
		return s.NextVersionToValidate(), TaskKindValidation // TxnVersion为空就直接返回空了，任务是啥都不要紧
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
	s.num_active_tasks.Add(1)
	idx_to_execute := s.execution_idx.Add(1) - 1
	return s.try_incarnate(TxnIndex(idx_to_execute))
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
	s.num_active_tasks.Add(1)
	idx_to_validate := s.validation_idx.Add(1) - 1
	if idx_to_validate < uint64(s.block_size) { // 再检查一遍，防止并发竞争
		entry := &s.txn_status[idx_to_validate]
		entry.Lock()
		if entry.status == StatusExecuted {
			incarnation := entry.incarnation
			entry.Unlock()
			return TxnVersion{TxnIndex(idx_to_validate), incarnation}
		}
		entry.Unlock()
	}

	s.num_active_tasks.Add(^uint64(0)) // 减1
	return InvalidTxnVersion
}

/*
function add_dependency(txn_idx, blocking_txn_idx)

	with txn_dependency[blocking_txn_idx].lock()
		if txn_status[blocking_txn_idx].lock().status = EXECUTED then ⊲ thread holds 2 locks
			return false ⊲ dependency resolved before locking in Line 148
		txn_status[txn_idx].lock().status() ← ABORTING ⊲ previous status must be EXECUTING
		txn_dependency[blocking_txn_idx].insert(txn_idx)
	num_active_tasks.decrement() ⊲ execution task aborted due to a dependency
	return true
*/
func (s *Scheduler) AddDependency(txnIdx TxnIndex, blockingTxnIdx TxnIndex) bool {
	dep := &s.txn_dependency[blockingTxnIdx]
	dep.Lock()
	defer dep.Unlock()

	// 持有 txn_dependency 锁的同时，再加 txn_status 锁（thread holds 2 locks）
	blockingStatus := &s.txn_status[blockingTxnIdx]
	blockingStatus.Lock()
	if blockingStatus.status == StatusExecuted {
		blockingStatus.Unlock()
		return false // dependency resolved before locking
	}
	blockingStatus.Unlock()

	// 将 txnIdx 的状态设为 ABORTING
	curStatus := &s.txn_status[txnIdx]
	curStatus.Lock()
	curStatus.status = StatusAborting
	curStatus.Unlock()

	// 将 txnIdx 加入 blockingTxnIdx 的依赖集
	dep.dependents = append(dep.dependents, txnIdx)

	// execution task aborted due to a dependency
	s.num_active_tasks.Add(^uint64(0)) // 减1
	return true
}

/*
procedure set_ready_status(txn_idx)

	with txn_status[txn_idx].lock()
		(incarnation_number, status) ← txn_status[txn_idx] ⊲ status must be ABORTING
		txn_status[txn_idx] ← (incarnation_number + 1, READY_TO_EXECUTE)
*/
func (s *Scheduler) SetReadyStatus(txnIdx TxnIndex) {
	entry := &s.txn_status[txnIdx]
	entry.Lock()
	// status must be ABORTING
	entry.incarnation++ // comment：再这里实现的incarnation++
	entry.status = StatusReadyToExecute
	entry.Unlock()
}

/*
procedure resume_dependencies(dependent_txn_indices)

	for each dep_txn_idx ∈ dependent_txn_indices do
		set_ready_status(dep_txn_idx)
	min_dependency_idx ← min(dependent_txn_indices) ⊲ minimum is ⊥ if no elements
	if min_dependency_idx ≠ ⊥ then
		decrease_execution_idx(min_dependency_idx) ⊲ ensure dependent indices get re-executed
*/
func (s *Scheduler) ResumeDependencies(dependents []TxnIndex) { // 输入是一群被阻塞的交易
	if len(dependents) == 0 {
		return
	}

	minIdx := dependents[0]
	for _, idx := range dependents {
		s.SetReadyStatus(idx)
		if idx < minIdx {
			minIdx = idx
		}
	}
	s.decrease_execution_idx(minIdx)
}

/*
procedure finish_execution(txn_idx, incarnation_number, wrote_new_path)

	txn_status[txn_idx].lock().status ← EXECUTED ⊲ status must have been EXECUTING
	deps ← txn_dependency[txn_idx].lock().swap({}) ⊲ swap out the set of dependent transaction indices
	resume_dependencies(deps)
	if validation_idx > txn_idx then ⊲ otherwise index already small enough
		if wrote_new_path then
			decrease_validation_idx(txn_idx) ⊲ schedule validation for txn_idx and higher txns
		else
		return (version ← (txn_idx, incarnation_number), kind ← VALIDATION_TASK  ⊲ 直接把这个验证任务返给自己，不修改全局的指针
	num_active_tasks.decrement()
	return ⊥				⊲ no task returned to the caller
*/
func (s *Scheduler) FinishExecution(version TxnVersion, wroteNewPath bool) (TxnVersion, TaskKind) {
	// txn_status[txn_idx].status ← EXECUTED
	entry := &s.txn_status[version.Index]
	entry.Lock()
	entry.status = StatusExecuted
	entry.Unlock()

	// deps ← txn_dependency[txn_idx].swap({})
	dep := &s.txn_dependency[version.Index]
	dep.Lock()
	deps := dep.dependents
	dep.dependents = nil
	dep.Unlock()

	// resume_dependencies(deps)
	s.ResumeDependencies(deps)

	// if validation_idx > txn_idx
	if s.validation_idx.Load() > uint64(version.Index) {
		if wroteNewPath {
			// decrease_validation_idx(txn_idx)
			s.decrease_validation_idx(version.Index)
		} else {
			// 直接把验证任务返给自己，不修改全局指针
			return version, TaskKindValidation // 优化点
		}
	}

	// num_active_tasks.decrement()
	s.num_active_tasks.Add(^uint64(0))
	return InvalidTxnVersion, TaskKindExecution // InvalidTxnVersion时，taskkind就不重要了
}

/*
function try_validation_abort(txn_idx, incarnation_number)

	with txn_status[txn_idx].lock()
		if txn_status[txn_idx] = (incarnation_number, EXECUTED) then
			txn_status[txn_idx].status ← ABORTING ⊲ thread changes status, starts aborting
			return true
	return false
*/
func (s *Scheduler) TryValidationAbort(version TxnVersion) bool {
	entry := &s.txn_status[version.Index]
	entry.Lock()
	defer entry.Unlock()

	if entry.incarnation == version.Incarnation && entry.status == StatusExecuted {
		entry.status = StatusAborting
		return true
	}
	return false
}

/*
procedure finish_validation(txn_idx, aborted)

	if aborted then
		set_ready_status(txn_idx)
		decrease_validation_idx(txn_idx + 1) ⊲ schedule validation for higher transactions
		if execution_idx > txn_idx then ⊲ otherwise index already small enough
			new_version ← try_incarnate(txn_idx)
			if new_version ≠ ⊥ then
				return (new_version, kind ← EXECUTION_TASK) ⊲ return re-execution task to the caller
	num_active_tasks.decrement() ⊲ done with validation task
	return ⊥ ⊲ no task returned to the caller
*/
func (s *Scheduler) FinishValidation(txnIdx TxnIndex, aborted bool) (TxnVersion, TaskKind) {
	if aborted {
		s.SetReadyStatus(txnIdx)
		s.decrease_validation_idx(txnIdx + 1) //没有任何序号大于 txnIdx 的事务能在这种不确定的情况下偷偷提交。而txnIdx应该重跑，还没到验证的时候

		if s.execution_idx.Load() > uint64(txnIdx) {
			newVersion := s.try_incarnate(txnIdx)
			if newVersion.Valid() {
				return newVersion, TaskKindExecution // 优化点
			}
			// try_incarnate 失败时已经 decrement 了 num_active_tasks，直接返回
			return InvalidTxnVersion, TaskKindExecution // 如果任务被别人领走了，就老老实实去领别的任务
		}
	}

	// done with validation task
	s.num_active_tasks.Add(^uint64(0))
	return InvalidTxnVersion, TaskKindExecution
}
