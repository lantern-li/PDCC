package reorder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/prometheus/client_golang/prometheus"

	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/utils"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
)

var (
	reexecRepeat   = 0
	skipSerialExec = false
)

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20
)

// ReorderTxScheduler deterministic transaction scheduler structure
type ReorderTxScheduler struct {
	lock            sync.Mutex
	scheduleFinishC chan bool
	log             protocol.Logger
	chainConf       protocol.ChainConf
	storeHelper     conf.StoreHelper
	metricVMRunTime *prometheus.HistogramVec
	// vmManager       protocol.VmManager
	// ledgerCache   protocol.LedgerCache
	// contractCache *sync.Map
	vmHelper *deterministic.CommonVMHelper // Shared VM execution helper
}

// NewReorderTxScheduler building a transaction scheduler
func NewReorderTxScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf,
	storeHelper conf.StoreHelper, ac protocol.AccessControlProvider,
) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.DebugDynamic(func() string {
		return "use the deterministic ReorderTxScheduler."
	})
	txScheduler := &ReorderTxScheduler{
		lock:            sync.Mutex{},
		scheduleFinishC: make(chan bool, 1),
		log:             log,
		chainConf:       chainConf,
		storeHelper:     storeHelper,
		// vmManager:       vmMgr,
		// ledgerCache:   cache,
		// contractCache: &sync.Map{},
	}
	// Initialize common VM helper
	txScheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return txScheduler
}

// Schedule schedules the transactions and returns the results of their execution.
func (ts *ReorderTxScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	txRwSet, contractEvents, err := ts.schedule(block, txBatch, snapshot)
	if err != nil {
		return nil, nil, err
	}
	return txRwSet, contractEvents, nil
}

// SimulateWithDag simulates the execution of transactions using DAG
func (ts *ReorderTxScheduler) SimulateWithDag(block *commonPb.Block,
	snapshot protocol.Snapshot,
) (map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	txRwSet, _, txResultMap, err := ts.simulate(block, block.Txs, snapshot)
	if err != nil {
		return nil, nil, err
	}
	return txRwSet, txResultMap, nil
}

// Halt schedule finish
func (ts *ReorderTxScheduler) Halt() {
	ts.scheduleFinishC <- true
}

// get Scheduler`s Pool Thread num
func (ts *ReorderTxScheduler) getSchedulerThreadNum(count int) int {
	//if ts.chainLocalConf.NodeConfig.SchedulerConfig.ScheduleMaxConcurrency > 0 {
	//	return int(ts.chainLocalConf.NodeConfig.SchedulerConfig.ScheduleMaxConcurrency)
	//}
	return count
}

// get Scheduler`s Pool Thread num
func (ts *ReorderTxScheduler) getSimulateThreadNum(txCount int) int {
	//if ts.chainLocalConf.NodeConfig.SchedulerConfig.SimulateMaxConcurrency > 0 {
	//	return int(ts.chainLocalConf.NodeConfig.SchedulerConfig.SimulateMaxConcurrency)
	//}
	return txCount
}

func (ts *ReorderTxScheduler) schedule(block *commonPb.Block, txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	defer ts.vmHelper.ReleaseContractCache()
	txBatchSize := len(txBatch)
	ts.log.Infof("schedule tx batch start, block_number = %v, size = %d", block.Header.BlockHeight, txBatchSize)

	var goRoutinePool *ants.Pool
	var err error
	poolCapacity := ts.storeHelper.GetPoolCapacity()
	ts.log.Debugf("GetPoolCapacity() => %v", poolCapacity)
	threadNum := ts.getSchedulerThreadNum(poolCapacity)
	ts.log.Debugf("getSchedulerThreadNum() => %v", threadNum)
	if goRoutinePool, err = ants.NewPool(threadNum, ants.WithPreAlloc(false)); err != nil {
		return nil, nil, err
	}
	defer goRoutinePool.Release()

	startTime := time.Now()

	// Record processed tx length
	processedTxBatchLen := make([]uint32, 0)
	// Pre-execute txs based on the same snapshot
	txResults := ts.preExecuteTxs(goRoutinePool, block, snapshot, txBatch, protocol.Schedule)

	timeCostA := time.Since(startTime)
	// Apply results based on reorder by removing strongly connected component
	txBatchLeft := ts.handleTxResults(snapshot, txResults, txBatch,
		func(txResults []*TxResultIndex) ([]*Vertex, bool) {
			readMap, writeMap := RWSet2Map(txResults)
			return BuildGraphRemoveSCC(readMap, writeMap, len(txResults))
		})

	timeCostB := time.Since(startTime)
	// Even if we processed 0 tx, we record it
	processedTxBatchLen = append(processedTxBatchLen, uint32(len(txBatch)-len(txBatchLeft)))
	// Repeat pre-execute and handle results for constant times
	for repeat := 0; repeat < reexecRepeat; repeat++ {
		if len(txBatchLeft) == 0 {
			// if no txs left, break out and go to serial execution
			break
		}
		ts.log.Infof("schedule tx batch start, block_number = %v, repeat = %d, size left = %d",
			block.Header.BlockHeight, repeat+1, len(txBatchLeft))
		txResults = ts.preExecuteTxs(goRoutinePool, block, snapshot, txBatchLeft, protocol.Schedule)
		// Apply results based on reorder by removing cycles
		txBatchLeft = ts.handleTxResults(snapshot, txResults, txBatchLeft,
			func(txResults []*TxResultIndex) ([]*Vertex, bool) {
				readMap, writeMap := RWSet2Map(txResults)
				return BuildGraphRemoveCycle(readMap, writeMap, len(txResults))
			})
		if len(txBatch)-len(txBatchLeft) == int(processedTxBatchLen[len(processedTxBatchLen)-1]) {
			// if no txs has been successfully handled in this loop, break out and go to serial execution
			break
		}
		// we record processed tx here
		processedTxBatchLen = append(processedTxBatchLen, uint32(len(txBatch)-len(txBatchLeft)))
	}
	// Serially execute left tx
	if len(txBatchLeft) > 0 && !skipSerialExec {
		ts.log.Infof("schedule tx batch start, fall back to serial, block_number = %v, size = %d",
			block.Header.BlockHeight, len(txBatchLeft))
		if err := ts.handleTxsSeriallyTimeout(txBatchLeft, snapshot, block, protocol.Schedule); err != nil {
			ts.log.Errorf("serial execution failed for block %d: %v", block.Header.BlockHeight, err)
		}
	}

	snapshot.Seal()
	timeCostC := time.Since(startTime)

	// write the dagdan
	block.Dag = &commonPb.DAG{Vertexes: []*commonPb.DAG_Neighbor{
		{Neighbors: processedTxBatchLen},
	}}

	// update block's txs(delete the tx which schedule time out.)
	// also delete tx which is removed during reorder.
	// now txs can be parallel-executed, and serial-applied
	block.Txs = snapshot.GetTxTable()

	timeCostD := time.Since(startTime)
	ts.log.Infof("schedule tx batch finished, success %d, txs pre-execution cost %v, "+
		"txs result process cost %v, repeat and serial cost %v, "+
		"dag building cost %v, total used %v, tps %v", len(block.Txs), timeCostA,
		timeCostB-timeCostA, timeCostC-timeCostB, timeCostD-timeCostC, timeCostD,
		float64(len(block.Txs))/(float64(timeCostD)/1e9))

	txRWSetMap := utils.GetTxRWSetTable(snapshot, block, ts.log)
	contractEventMap := utils.GetContractEventMap(block)

	return txRWSetMap, contractEventMap, nil
}

// use go routine to parallel pre-execute (预执行)
// preExecuteTxs must be called with ts.lock held
// Only one invocation can run at a time
func (ts *ReorderTxScheduler) preExecuteTxs(goRoutinePool *ants.Pool,
	block *commonPb.Block, snapshot protocol.Snapshot,
	txBatch []*commonPb.Transaction, mode protocol.ScheduleMode,
) []*TxResultIndex {
	txBatchSize := len(txBatch)
	runningTxC := make(chan *TxIndex, txBatchSize)
	txResultC := make(chan *TxResultIndex, txBatchSize)
	txResults := make([]*TxResultIndex, txBatchSize)
	ctx, cancel := context.WithTimeout(context.Background(), ScheduleTimeout*time.Second)
	defer cancel()
	// launch the go routine to dispatch tx to runningTxC
	// goroutine 1: 分发交易（支持取消）
	go func() {
		defer close(runningTxC) // ← 重要：关闭 channel
		for i, tx := range txBatch {
			select {
			case <-ctx.Done():
				return // ← 支持取消
			case runningTxC <- &TxIndex{Index: i, Tx: tx}:
			}
		}
	}()
	// Put the pending transaction into the running queue and process results and timeouts
	// goroutine 2: 处理交易
	go func() {
		counter := 0
		for {
			select {
			case txI, ok := <-runningTxC:
				if !ok { // ← channel 已关闭
					return
				}
				// additional timeoutC check to avoid timeoutC starving,避免单笔交易，导致无法退出
				select {
				case <-ctx.Done():
					ts.scheduleFinishC <- true
					ts.log.Warnf("block [%d] schedule reached time limit",
						block.Header.BlockHeight)
					return
				default:
				}
				ts.log.Debugf("prepare to submit running task for tx id:%s",
					txI.Tx.Payload.GetTxId())

				err := goRoutinePool.Submit(func() {
					select {
					case <-ctx.Done():
						return // 提前退出
					default:
					}
					txSimContext, specialTxType, runTxSuccess := ts.vmHelper.ExecuteTx(txI.Tx, snapshot, block)
					if mode == protocol.Schedule {
						txI.Tx.Result = txSimContext.GetTxResult()
					}
					txResultC <- &TxResultIndex{
						Index:   txI.Index,
						Sim:     txSimContext,
						TxType:  specialTxType,
						Success: runTxSuccess,
					}
					ts.log.DebugDynamic(func() string {
						return fmt.Sprintf("handleTx(`%v`) => ExecuteTx(...) => runTxSuccess = %v",
							txI.Tx.GetPayload().TxId, runTxSuccess)
					})
				})
				if err != nil {
					ts.log.Warnf("failed to submit running task, tx id:%s during schedule, %+v",
						txI.Tx.Payload.GetTxId(), err)
					txResultC <- &TxResultIndex{Index: txI.Index, Sim: nil}
				}
			case <-ctx.Done():
				ts.scheduleFinishC <- true
				ts.log.Warnf("block [%d] schedule reached time limit", block.Header.BlockHeight)
				return
			case txR := <-txResultC:
				if txR.Sim != nil {
					txResults[txR.Index] = txR
				}
				counter++
				ts.log.Debugf("schedule tx index %d, count %d", txR.Index, counter)
				if counter == txBatchSize {
					ts.scheduleFinishC <- true
					return
				}
			}
		}
	}()

	// Wait for schedule finish signal
	<-ts.scheduleFinishC
	// Return slice of results
	return txResults
}

// return tx left, that are not handled and applied
func (ts *ReorderTxScheduler) handleTxResults(snapshot protocol.Snapshot,
	txResults []*TxResultIndex, txBatch []*commonPb.Transaction,
	buildDagFn func([]*TxResultIndex) ([]*Vertex, bool),
) []*commonPb.Transaction {
	startTime := time.Now()
	graph, earlyReturn := buildDagFn(txResults)
	if earlyReturn {
		return txBatch
	}
	timeCostA := time.Since(startTime)
	// now graph is DAG
	// if we miss any tx result, we should remove it from the graph
	for i, txResultI := range txResults {
		if txResultI == nil {
			ts.log.Warnf("schedule tx %d timeout, remove it from final DAG", i)
			graph[i].Removed = true
		}
	}
	// get topological order
	topo := NewTopologicalOrder(graph)
	reorder := topo.Run()
	timeCostB := time.Since(startTime)
	for _, i := range reorder {
		txResultI := txResults[i]
		// note: applySpecialTx = true
		applyResult, applySize := snapshot.ApplyTxSimContext(txResultI.Sim, txResultI.TxType, txResultI.Success, true)
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => ApplyTxSimContext(...) => snapshot.txTable = %v, applySize = %v",
				txResultI.Sim.GetTx().GetPayload().TxId, len(snapshot.GetTxTable()), applySize)
		})
		if !applyResult {
			ts.log.Errorf("apply to snapshot failed, this should not happen in reorder scheduler, "+
				"tx id:%s, result:%+v, apply count:%d", txResultI.Sim.GetTx().Payload.GetTxId(),
				txResultI.Sim.GetTxResult(), txResultI.Index)
		}
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("apply to snapshot success, tx id:%s, result:%+v, apply count:%d",
				txResultI.Sim.GetTx().Payload.GetTxId(), txResultI.Sim.GetTxResult(), applySize)
		})
	}
	var txsLeft []*commonPb.Transaction
	for i, v := range graph {
		if v.Removed {
			txsLeft = append(txsLeft, txBatch[i])
		}
	}
	timeCostC := time.Since(startTime)
	ts.log.Infof("reorder graph cost %v, reorder cost %v, apply cost %v",
		timeCostA, timeCostB-timeCostA, timeCostC-timeCostB)
	return txsLeft
}

func (ts *ReorderTxScheduler) handleTxsSerially(ctx context.Context, txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot, block *commonPb.Block, mode protocol.ScheduleMode,
) error {
	for _, tx := range txBatch {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			ts.log.Warnf("handleTxsSerially cancelled: %v", ctx.Err())
			return ctx.Err()
		default:
			// Continue processing
		}

		//var start time.Time
		//if ts.chainLocalConf.NodeConfig.Monitor.Enable {
		//	start = time.Now()
		//}

		// execute tx, and get
		// 1) the read/write set
		// 2) the result that telling if the invoke success.
		txSimContext, specialTxType, runTxSuccess := ts.vmHelper.ExecuteTx(tx, snapshot, block)

		if mode == protocol.Schedule {
			tx.Result = txSimContext.GetTxResult()
		}
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => ExecuteTx(...) => runTxSuccess = %v", tx.GetPayload().TxId, runTxSuccess)
		})

		// Apply failed means this tx's read set conflict with other txs' write set
		// note: applySpecialTx = true
		applyResult, applySize := snapshot.ApplyTxSimContext(txSimContext, specialTxType,
			runTxSuccess, true)
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => ApplyTxSimContext(...) => snapshot.txTable = %v, applySize = %v",
				tx.GetPayload().TxId, len(snapshot.GetTxTable()), applySize)
		})

		if !applyResult {
			ts.log.Errorf("serial handle should not have conflict, "+
				"this should not happen in reorder scheduler, tx id:%s, result:%+v, apply count:%d",
				tx.Payload.GetTxId(), txSimContext.GetTxResult(), applySize)
		}

		//if ts.chainLocalConf.NodeConfig.Monitor.Enable {
		//	elapsed := time.Since(start)
		//	ts.metricVMRunTime.WithLabelValues(tx.Payload.ChainId).Observe(elapsed.Seconds())
		//}

		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("apply to snapshot success, tx id:%s, result:%+v, apply count:%d",
				tx.Payload.GetTxId(), txSimContext.GetTxResult(), applySize)
		})
	}
	return nil
}

func (ts *ReorderTxScheduler) handleTxsSeriallyTimeout(txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot, block *commonPb.Block, mode protocol.ScheduleMode,
) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ScheduleWithDagTimeout*time.Second)
	defer cancel()

	finishC := make(chan bool, 1)
	errC := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic occurred, send error
				errC <- fmt.Errorf("panic in handleTxsSerially: %v", r)
				return
			}
		}()
		// Execute transactions serially
		if err := ts.handleTxsSerially(ctx, txBatch, snapshot, block, mode); err != nil {
			errC <- err
		} else {
			finishC <- true
		}
	}()

	// Wait for finish or timeout
	select {
	case err := <-errC:
		return err
	case <-ctx.Done():
		// Timeout with error
		return fmt.Errorf("schedule timeout after %d seconds", ScheduleWithDagTimeout)
	case <-finishC:
		return nil
	}
}

func (ts *ReorderTxScheduler) handleTxResultsSerially(snapshot protocol.Snapshot, txResults []*TxResultIndex) {
	for i, txResultI := range txResults {
		if txResultI == nil {
			ts.log.Warnf("schedule tx %d has no result, tx result and rwset check will not pass", i)
			continue
		}
		// note: applySpecialTx = true
		applyResult, applySize := snapshot.ApplyTxSimContext(txResultI.Sim, txResultI.TxType, txResultI.Success, true)
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => ApplyTxSimContext(...) => snapshot.txTable = %v, applySize = %v",
				txResultI.Sim.GetTx().GetPayload().TxId, len(snapshot.GetTxTable()), applySize)
		})
		if !applyResult {
			ts.log.Errorf("apply to snapshot failed, this should not happen in reorder scheduler, "+
				"tx id:%s, result:%+v, apply count:%d", txResultI.Sim.GetTx().Payload.GetTxId(),
				txResultI.Sim.GetTxResult(), txResultI.Index)
		}
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("apply to snapshot success, tx id:%s, result:%+v, apply count:%d",
				txResultI.Sim.GetTx().Payload.GetTxId(), txResultI.Sim.GetTxResult(), applySize)
		})
	}
}

// use DAG to simulate preExecutes
func (ts *ReorderTxScheduler) simulate(block *commonPb.Block,
	txBatch []*commonPb.Transaction, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet,
	map[string][]*commonPb.ContractEvent, map[string]*commonPb.Result, error,
) {
	if block.Dag == nil || len(block.Dag.Vertexes) == 0 {
		return nil, nil, nil, fmt.Errorf("no metadata in DAG for scheduler simulate")
	}

	ts.lock.Lock()
	defer ts.lock.Unlock()
	defer ts.vmHelper.ReleaseContractCache()
	var goRoutinePool *ants.Pool
	var err error
	poolCapacity := ts.storeHelper.GetPoolCapacity()
	ts.log.Debugf("GetPoolCapacity() => %v", poolCapacity)
	threadNum := ts.getSimulateThreadNum(poolCapacity)
	ts.log.Debugf("getSimulateThreadNum() => %v", threadNum)

	if goRoutinePool, err = ants.NewPool(threadNum, ants.WithPreAlloc(false)); err != nil {
		return nil, nil, nil, err
	}
	defer goRoutinePool.Release()

	// Pre-execute txs based on the dag
	lastBatchSize := uint32(0)
	for i, txBatchSize := range block.Dag.Vertexes[0].Neighbors {
		if txBatchSize <= uint32(len(txBatch)) && lastBatchSize < txBatchSize {
			ts.log.DebugDynamic(func() string {
				return fmt.Sprintf("block [%d] simulate batch %d size:%d, total batch size:%d",
					block.Header.BlockHeight, i, txBatchSize-lastBatchSize, len(txBatch))
			})
			txResults := ts.preExecuteTxs(goRoutinePool, block, snapshot, txBatch[lastBatchSize:txBatchSize], protocol.Simulate)
			ts.handleTxResultsSerially(snapshot, txResults)
			lastBatchSize = txBatchSize
		}
	}
	txBatchSize := uint32(len(txBatch))
	// If we skip serial execution of last few txs, then two number should equal
	if skipSerialExec && lastBatchSize != txBatchSize {
		return nil, nil, nil, fmt.Errorf("metadata in DAG last size should equal tx count")
	}
	// Handle the last txs serially
	if lastBatchSize < txBatchSize {
		if err := ts.handleTxsSeriallyTimeout(txBatch[lastBatchSize:txBatchSize], snapshot, block,
			protocol.Simulate); err != nil {
			ts.log.Errorf("serial execution failed in simulate for block %d: %v", block.Header.BlockHeight, err)
		}
	}

	snapshot.Seal()

	txRWSetMap := utils.GetTxRWSetTable(snapshot, block, ts.log)
	contractEventMap := utils.GetContractEventMap(block)

	return txRWSetMap, contractEventMap, snapshot.GetTxResultMap(), nil
}
