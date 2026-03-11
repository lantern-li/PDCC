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
	tx                    *commonPb.Transaction
	txSimContext          protocol.TxSimContext
	txRWSet               *commonPb.TxRWSet
	txReadSet             []*commonPb.TxRead
	txWriteSetWithVersion []*commonPb.VersionedTxWrite
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
		batchSize:   200,                                // comment：Aria的批大小是固定的，这里就固定为200
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
	As.log.Infof("Graph schedule start, block_number = %v, tx_count = %d, batchsize = %d", block.Header.BlockHeight, len(txBatch), As.batchSize) //动态调整后整理的ws.batchSize要改

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
					txSimContext: txSimContext,
					txRWSet:      txRWSet,
					txReadSet:    txRWSet.TxReads,
				}

				// 对写集合进行版本标记。 comment：Graph没有重排序，所以这时就可以对写集进行版本的处理
				versionedWrites := make([]*commonPb.VersionedTxWrite, 0, len(txRWSet.TxWrites))

				for _, w := range txRWSet.TxWrites {
					versionedWrites = append(versionedWrites, &commonPb.VersionedTxWrite{
						Write:   w,
						Version: uint64(idx),
					})
				}

				execInfos[idx].txWriteSetWithVersion = versionedWrites
			}(i, tx)
		}
		wg.Wait()

		As.log.DebugDynamic(func() string {
			return fmt.Sprintf("[ExecutionStage] execute %d txs finished, total cost=%v", len(selectedTxs), time.Since(execStageStart))
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
