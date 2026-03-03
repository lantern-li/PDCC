/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package graph

import (
	"runtime"
	"sync"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20
	// DefaultBatchSizeMultiplier 默认批处理大小倍数（相对于CPU核心数）
	DefaultBatchSizeMultiplier = 10
)

// GraphScheduler A deterministic parallel scheduler
type GraphScheduler struct {
	lock           sync.Mutex
	log            protocol.Logger
	chainConf      protocol.ChainConf
	storeHelper    conf.StoreHelper
	vmHelper       *deterministic.CommonVMHelper // Shared VM execution helper
	snapshotCache  sync.Map                      // key: string(Write.Key), value: *commonPb.VersionedTxWrite
	txRWSetMap     map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet  todo chainmaker的这个也要改
	txRWSetMapLock sync.Mutex                    // lock for txRWSetMap concurrent access
	batchSize      int                           // 批处理大小，从配置文件读取或使用默认值
}

// NewGraphSchedulerr creates a new WRIA transaction scheduler
func NewGraphScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.DebugDynamic(func() string {
		return "use the deterministic WRIA scheduler"
	})

	// 从配置文件读取 batch_size，如果未配置则使用默认值（CPU核心数 * 10）
	batchSize := int(chainConf.ChainConfig().Scheduler.GetBatchSize())
	if batchSize <= 0 {
		batchSize = runtime.NumCPU() * DefaultBatchSizeMultiplier
		log.Infof("BatchSize not configured, using default value: %d (NumCPU=%d * %d)",
			batchSize, runtime.NumCPU(), DefaultBatchSizeMultiplier)
	} else {
		batchSize = runtime.NumCPU() * batchSize
		log.Infof("BatchSize configured from chain config: %d", batchSize)
	}

	scheduler := &GraphScheduler{
		lock:        sync.Mutex{},
		log:         log,
		chainConf:   chainConf,
		storeHelper: storeHelper,
		txRWSetMap:  make(map[string]*commonPb.TxRWSet), // 初始化 txRWSetMap
		batchSize:   batchSize,                          // 设置批处理大小
		// snapshotCache sync.Map 不需要初始化
	}

	scheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return scheduler
}

// 说明：v2.3.5之后，交易执行时，如果从自己的写集中读取，那么这个读集不会被记录到最终txSimContext中的读集中

// Schedule schedules the transactions using WRIA algorithm.
func (Gs *GraphScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {

	return Gs.txRWSetMap, nil, nil
}

// SimulateWithDag simulates the execution of transactions using the DAG in the block.
func (Gs *GraphScheduler) SimulateWithDag(block *commonPb.Block, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	Gs.lock.Lock()
	defer Gs.lock.Unlock()
	defer Gs.vmHelper.ReleaseContractCache()

	Gs.log.Infof("WRIA simulate with DAG start, block_number = %v, tx_count = %d",
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
func (Gs *GraphScheduler) Halt() {
	// todo
}
