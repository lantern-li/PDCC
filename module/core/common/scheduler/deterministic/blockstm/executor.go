package blockstm

import "context"

// Executor fields are not mutated during execution.
type Executor struct {
	ctx       context.Context // context for cancellation
	scheduler *Scheduler      // scheduler for task management
	//txExecutor TxExecutor      // callback to actually execute a transaction todo:maybe refactor no need
	//mvMemory   *MVMemory       // multi-version memory for the executor todo

	// index of the executor, used for debugging output
	i int
}

func NewExecutor(
	ctx context.Context,
	scheduler *Scheduler,
	//txExecutor TxExecutor,
	//mvMemory *MVMemory,
	i int,
) *Executor {
	return &Executor{
		ctx:       ctx,
		scheduler: scheduler,
		//txExecutor: txExecutor,
		//mvMemory:   mvMemory,
		i: i,
	}
}

// Invariant `num_active_tasks`:
//   - `NextTask` increases it if returns a valid task.
//   - `TryExecute` and `NeedsReexecution` don't change it if it returns a new valid task to run,
//     otherwise it decreases it.
func (e *Executor) Run() {
	var kind TaskKind
	version := InvalidTxnVersion
	for !e.scheduler.Done() {
		if !version.Valid() {
			// check for cancellation
			select {
			case <-e.ctx.Done():
				return
			default:
			}

			version, kind = e.scheduler.NextTask()
			continue
		}

		switch kind {
		case TaskKindExecution:
			version, kind = e.TryExecute(version)
		case TaskKindValidation:
			version, kind = e.NeedsReexecution(version)
		}
	}
}
