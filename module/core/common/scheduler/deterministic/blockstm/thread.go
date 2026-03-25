package blockstm

import (
	"context"
	"errors"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// Thread fields are not mutated during execution.
type Thread struct {
	ctx       context.Context
	scheduler *Scheduler
	mvMemory  *MVMemory
	log       protocol.Logger

	// 执行一笔交易所需
	txBatch  []*commonPb.Transaction // 把交易的执行放到线程器中，不包装多余的东西
	vmHelper *deterministic.CommonVMHelper
	snapshot protocol.Snapshot
	block    *commonPb.Block

	i int // 线程器的标识符
}

func NewThread(
	ctx context.Context,
	scheduler *Scheduler,
	mvMemory *MVMemory,
	txBatch []*commonPb.Transaction,
	vmHelper *deterministic.CommonVMHelper,
	snapshot protocol.Snapshot,
	block *commonPb.Block,
	i int,
	log protocol.Logger,
) *Thread {
	return &Thread{
		ctx:       ctx,
		scheduler: scheduler,
		mvMemory:  mvMemory,
		txBatch:   txBatch,
		vmHelper:  vmHelper,
		snapshot:  snapshot,
		block:     block,
		i:         i,
		log:       log,
	}
}

// Invariant `num_active_tasks`:
//   - `NextTask` increases it if returns a valid task.
//   - `TryExecute` and `NeedsReexecution` don't change it if it returns a new valid task to run,
//     otherwise it decreases it.

/*
procedure run()
	task ← ⊥
	while ¬Scheduler.done() do
		if task ≠ ⊥ ∧ task.kind = EXECUTION_TASK then
			task ← try_execute(task.version) ⊲ returns a validation task, or ⊥
		if task ≠ ⊥ ∧ task.kind = VALIDATION_TASK then
			task ← needs_reexecution(task.version) ⊲ returns a re-execution task, or ⊥
		if task = ⊥ then
			task ← Scheduler.next_task()
*/

func (t *Thread) Run() {
	// task ← ⊥
	version := InvalidTxnVersion
	var kind TaskKind

	for !t.scheduler.Done() {
		if version.Valid() && kind == TaskKindExecution {
			version, kind = t.TryExecute(version)
		}
		if version.Valid() && kind == TaskKindValidation {
			version, kind = t.NeedsReexecution(version)
		}
		if !version.Valid() {
			select {
			case <-t.ctx.Done():
				return
			default:
			}
			version, kind = t.scheduler.NextTask()
		}
	}
}

/*
function try_execute(version)
	(txn_idx, incarnation_number) ← version
	vm_result ← VM.execute(txn_idx) ⊲ VM does not write to shared memory
	if vm_result.status = READ_ERROR then
		if ¬Scheduler.add_dependency(txn_idx, vm_result.blocking_txn_idx) then
			return try_execute(version) ⊲ dependency resolved in the meantime, re-execute
		return ⊥
	else
		wrote_new_location ← MVMemory.record(version, vm_result.read_set, vm_result.write_set）
		return Scheduler.finish_execution(txn_idx, incarnation_number, wrote_new_location)
*/

func (t *Thread) TryExecute(version TxnVersion) (TxnVersion, TaskKind) {
	for {
		readSet, writeSet, txSimContext, blockingIdx, isReadError := t.Execute(version)
		if isReadError {
			// dependency resolved in the meantime → re-execute
			if !t.scheduler.AddDependency(version.Index, blockingIdx) {
				continue
			}
			return InvalidTxnVersion, TaskKindExecution // comment：交易成功被AddDependency后，execution_idx没有减少。等依赖被解决后，会被设置SetReadyStatus，并且尝试回拨execution_idx
		}
		wroteNewLocation := t.mvMemory.record(version, readSet, writeSet, txSimContext)
		return t.scheduler.FinishExecution(version, wroteNewLocation)
	}
}

/*
function needs_reexecution(version) ⊲ returns a task for re-execution, or ⊥

	(txn_idx, incarnation_number) ← version
	read_set_valid ← MVMemory.validate_read_set(txn_idx)
	aborted ← ¬read_set_valid ∧ Scheduler.try_validation_abort(txn_idx, incarnation_number)
	if aborted then
		MVMemory.convert_writes_to_estimates(txn_idx)
	return Scheduler.finish_validation(txn_idx, aborted)
*/
func (t *Thread) NeedsReexecution(version TxnVersion) (TxnVersion, TaskKind) {
	readSetValid := t.mvMemory.validate_read_set(version.Index)
	aborted := !readSetValid && t.scheduler.TryValidationAbort(version) //if abort_validation returns false, then the incanation was already aborted.
	if aborted {
		t.mvMemory.convert_writes_to_estimates(version.Index)
	}
	return t.scheduler.FinishValidation(version.Index, aborted)
}

func (t *Thread) Execute(version TxnVersion) (readSet ReadSet, writeSet []write_set, txSimContext protocol.TxSimContext, blockingIdx TxnIndex, isReadError bool) {

	tx := t.txBatch[version.Index]

	mvSnap := newMVSnapshot(t.snapshot, t.mvMemory, version.Index)

	txSimContext, _, runVmSuccess, err := t.vmHelper.ExecuteTxForBlockSTM(tx, mvSnap, t.block)
	if err != nil {
		var blocked interface{ BlockingTxnIndex() int }
		if errors.As(err, &blocked) {
			idx := blocked.BlockingTxnIndex()
			// Defensive bounds check: the "read blocked" marker may come from a stringified VM error.
			// Ensure it cannot crash the scheduler by indexing outside the current block range.
			if idx >= 0 && idx < int(version.Index) && idx < len(t.txBatch) {
				t.log.Warnf("[BlockSTM] tx[%d] read blocked by tx[%d]", version.Index, idx)
				return ReadSet{}, nil, nil, TxnIndex(idx), true
			}
		}
	}

	readSet = mvSnap.ReadSet()

	txRWSet := txSimContext.GetTxRWSet(runVmSuccess)
	writeSet = convertTxRWSetToWriteSet(txRWSet)

	return readSet, writeSet, txSimContext, 0, false
}

func convertTxRWSetToWriteSet(txRWSet *commonPb.TxRWSet) []write_set {
	if txRWSet == nil || len(txRWSet.TxWrites) == 0 {
		return nil
	}

	out := make([]write_set, 0, len(txRWSet.TxWrites))
	for _, w := range txRWSet.TxWrites {
		// SQL-related writes may not have a stable key for MVMemory locationing.
		// Skip nil keys to avoid collapsing many writes into the same location.
		if w.Key == nil {
			continue
		}
		loc := constructKey(w.ContractName, w.Key)
		out = append(out, write_set{
			location: loc,
			value:    w.Value,
		})
	}
	return out
}
