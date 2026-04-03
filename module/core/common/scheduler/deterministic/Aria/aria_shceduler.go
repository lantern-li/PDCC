/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package aria

import (
	"fmt"
	"sync"
	"time"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20
)

// txExecInfo 存储交易执行的相关信息
type txExecInfo struct {
	tx           *commonPb.Transaction
	index        int
	txSimContext protocol.TxSimContext
	txRWSet      *commonPb.TxRWSet
	txReadSet    []*commonPb.TxRead
	txWriteSet   []*commonPb.TxWrite
}

// AriaScheduler A deterministic parallel scheduler
type AriaScheduler struct {
	lock           sync.Mutex
	log            protocol.Logger
	chainConf      protocol.ChainConf
	storeHelper    conf.StoreHelper
	vmHelper       *deterministic.CommonVMHelper // Shared VM execution helper
	snapshotCache  sync.Map                      // key: string(Write.Key), value: *commonPb.VersionedTxWrite
	txRWSetMap     map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet  todo chainmaker用这个落库。
	txRWSetMapLock sync.Mutex                    // lock for txRWSetMap concurrent access todo 这个似乎没用上
	batchSize      int                           // 批处理大小，从配置文件读取或使用默认值
}

// NewAriaScheduler creates a new Aria transaction scheduler
func NewAriaScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Infof("use the deterministic Aria scheduler")

	scheduler := &AriaScheduler{
		log:         log,
		chainConf:   chainConf,
		storeHelper: storeHelper,
		txRWSetMap:  make(map[string]*commonPb.TxRWSet), // 初始化 txRWSetMap
		batchSize:   100,                                // comment：Aria的批大小是固定的，这里固定为100
		// snapshotCache sync.Map 不需要初始化
	}

	scheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return scheduler
}

// 说明：v2.3.5之后，交易执行时，如果从自己的写集中读取，那么这个读集不会被记录到最终txSimContext中的读集中

// Schedule schedules the transactions using Graph algorithm.
func (As *AriaScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	As.lock.Lock()
	defer As.lock.Unlock()
	defer As.vmHelper.ReleaseContractCache()
	As.log.Infof("Aria schedule start, block_number = %v, tx_count = %d, batchsize = %d", block.Header.BlockHeight, len(txBatch), As.batchSize) //动态调整后整理的ws.batchSize要改

	As.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 循环调度，直到 txBatch 为空或超时
	startTime := time.Now()
	timeoutDuration := time.Duration(ScheduleTimeout) * time.Second
	roundNum := 0

	for len(txBatch) > 0 {
		roundNum++

		// 检查超时
		if time.Since(startTime) > timeoutDuration {
			As.log.Warnf("Schedule timeout after %d rounds, remaining %d transactions", roundNum, len(txBatch))
			break
		}

		//1. 选择阶段：依据BatchSize从当前txBatch中选取前BatchSize个序号最小交易进行调度，txBatch为剩余交易池
		batchSize := As.batchSize
		if batchSize > len(txBatch) {
			batchSize = len(txBatch)
		}
		selectedTxs := make([]*commonPb.Transaction, batchSize)
		copy(selectedTxs, txBatch[:batchSize]) // copy后，selectedTxs是新的底层数组
		txBatch = txBatch[batchSize:]          // 这里还是引用原有的底层数组，只不过指针变了
		As.log.DebugDynamic(func() string {
			return fmt.Sprintf("Round %d: selected %d transactions for scheduling, remainTxs=%d", roundNum, batchSize, len(txBatch))
		})

		// 2. 执行阶段：将selectedTxs并发地在当前相同的snapshot上执行。
		var wg sync.WaitGroup
		execInfos := make([]txExecInfo, len(selectedTxs)) // comment：注意这里是值引用

		execStageStart := time.Now() // 记录开始时间

		for i, tx := range selectedTxs { // i 从 0 开始
			wg.Add(1)
			// 每个 goroutine 只写 execInfos[idx] 这个唯一槽位，且 idx 不重复。
			go func(idx int, transaction *commonPb.Transaction) {
				defer wg.Done()

				txSimContext, _, runTxSuccess := As.vmHelper.ExecuteTx(transaction, snapshot, block) // comment：都在相同的snapshot上执行

				txRWSet := txSimContext.GetTxRWSet(runTxSuccess)

				execInfos[idx] = txExecInfo{
					tx:           transaction,
					index:        idx,
					txSimContext: txSimContext,
					txRWSet:      txRWSet,
					txReadSet:    txRWSet.TxReads,
					txWriteSet:   txRWSet.TxWrites,
				}
			}(i, tx)
		}
		wg.Wait()

		As.log.DebugDynamic(func() string {
			return fmt.Sprintf("[ExecutionStage] execute %d txs finished, total cost=%v", len(selectedTxs), time.Since(execStageStart))
		})

		// 3.1 生成写预留表 ReserveWrite
		reserveTable, aborted := reserveWrite(execInfos)
		// 3.2 生成读预留表 ReserveRead
		readReserveTable := reserveRead(execInfos)

		// 统计 abort 的交易数
		abortCount := 0
		for i := range aborted {
			if aborted[i].Load() {
				abortCount++
			}
		}
		As.log.DebugDynamic(func() string {
			return fmt.Sprintf("[ReserveWriteStage] round %d: %d txs aborted out of %d",
				roundNum, abortCount, len(execInfos))
		})

		// 4. 冲突检查阶段：基于预留表检查 WAW 和 RAW 依赖
		// 已在写预留阶段被 abort 的交易会自动跳过（性能优化）
		checkConflictStart := time.Now()
		checkConflicts(execInfos, reserveTable, readReserveTable, aborted, snapshot)

		As.log.DebugDynamic(func() string {
			// 重新统计（checkConflicts 可能新增了 abort）
			finalAbortCount := 0
			for i := range aborted {
				if aborted[i].Load() {
					finalAbortCount++
				}
			}
			return fmt.Sprintf("[CheckConflictStage] round %d: %d txs aborted out of %d (reserve=%d, conflict=%d), cost=%v",
				roundNum, finalAbortCount, len(execInfos), abortCount, finalAbortCount-abortCount, time.Since(checkConflictStart))
		})

		// 5. 提交/回退阶段：按序分离已提交和需重试的交易
		abortedTxs := make([]*commonPb.Transaction, 0)
		committedCount := 0
		for i := range execInfos {
			if aborted[i].Load() {
				abortedTxs = append(abortedTxs, execInfos[i].tx) // comment：abort交易从小到大收集，并放入下一批的头部
			} else {
				// 提交：记录结果，加入 block.Txs，存储读写集 comment:所以可提交交易必定可以等价于某种串行执行顺序
				execInfos[i].tx.Result = execInfos[i].txSimContext.GetTxResult()
				block.Txs = append(block.Txs, execInfos[i].tx) // comment：这里注意block.Txs给出的不是该调度的可串行化顺序！但是因为按照同样的从小到大的顺序放置，原来的确定性验证也能通过。
				As.txRWSetMap[execInfos[i].tx.Payload.TxId] = execInfos[i].txRWSet
				committedCount++
			}
		}

		// 将被 abort 的交易放回 txBatch 头部，下一轮重新执行
		if len(abortedTxs) > 0 {
			txBatch = append(abortedTxs, txBatch...)
		}

		As.log.DebugDynamic(func() string {
			return fmt.Sprintf("[CommitStage] round %d: committed=%d, aborted=%d",
				roundNum, committedCount, len(abortedTxs))
		})
	}

	totalTime := time.Since(startTime)
	tps := float64(len(block.Txs)) / totalTime.Seconds()
	As.log.Infof("Aria schedule completed after %d rounds, total time=%v, total txs=%d, TPS=%.2f, blockheight=%d",
		roundNum, totalTime, len(block.Txs), tps, block.Header.BlockHeight)

	return As.txRWSetMap, nil, nil
}

// SimulateWithDag simulates the execution of transactions using the DAG in the block.
func (As *AriaScheduler) SimulateWithDag(block *commonPb.Block, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	As.lock.Lock()
	defer As.lock.Unlock()
	defer As.vmHelper.ReleaseContractCache()

	As.log.Infof("WRIA simulate with DAG start, block_number = %v, tx_count = %d",
		block.Header.BlockHeight, len(block.Txs))

	// TODO: Implement DAG-based simulation
	// Use the DAG information from the block to execute transactions

	//txRwSetMap, txResultMap, err := ws.simulateWithDag(block, snapshot)

	//if err != nil {
	//	ws.log.Errorf("WRIA simulate with DAG failed: %v", err)
	//	return nil, nil, err
	//}
	//
	//ws.log.Infof("WRIA simulate with DAG finish, block_number = %v", block.Header.BlockHeight)

	return nil, nil, nil
}

// Halt stops the scheduler and releases resources
func (As *AriaScheduler) Halt() {
	// todo
}
