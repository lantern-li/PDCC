package blockstm

import (
	"context"
	"runtime"
	"sync"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

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

// BlockScheduler A deterministic parallel scheduler
type BlockScheduler struct {
	lock           sync.Mutex
	log            protocol.Logger
	chainConf      protocol.ChainConf
	storeHelper    conf.StoreHelper
	vmHelper       *deterministic.CommonVMHelper // Shared VM execution helper
	txRWSetMap     map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet  todo chainmaker用这个落库。
	txRWSetMapLock sync.Mutex                    // lock for txRWSetMap concurrent access todo 这个似乎没用上

	executors int // 线程数量 comment：每个 executors 可以执行 execute 任务，也可以执行 validate 任务
}

// NewBlockStmScheduler creates a new BlockStm transaction scheduler
func NewBlockStmScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Infof("use the deterministic BlockStm scheduler")

	scheduler := &BlockScheduler{
		log:         log,
		chainConf:   chainConf,
		storeHelper: storeHelper,
		txRWSetMap:  make(map[string]*commonPb.TxRWSet), // 初始化 txRWSetMap

		executors: runtime.NumCPU(), // todo:确认是否为论文建议的配置数
	}

	scheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return scheduler
}

// 说明：v2.3.5之后，交易执行时，如果从自己的写集中读取，那么这个读集不会被记录到最终txSimContext中的读集中

// Schedule schedules the transactions using Graph algorithm.
func (Bs *BlockScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	Bs.lock.Lock()
	defer Bs.lock.Unlock()
	defer Bs.vmHelper.ReleaseContractCache()

	Bs.log.Infof("BlockSTM schedule started, block_number = %v, tx_count = %d", block.Header.BlockHeight, len(txBatch))
	Bs.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 创建调度器
	scheduler := NewScheduler(len(txBatch))
	// todo 多版本内存

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(Bs.executors)
	for i := 0; i < Bs.executors; i++ {
		e := NewExecutor(ctx, scheduler, i) // 创建每个执行器
		go func() {
			defer wg.Done()
			e.Run() // 让每个执行器跑起来
		}()
	}
	wg.Wait()

	return Bs.txRWSetMap, nil, nil
}

// SimulateWithDag simulates the execution of transactions using the DAG in the block.
func (Bs *BlockScheduler) SimulateWithDag(block *commonPb.Block, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	Bs.lock.Lock()
	defer Bs.lock.Unlock()
	defer Bs.vmHelper.ReleaseContractCache()
	Bs.log.Infof("BlockStm simulate with DAG start, block_number = %v, tx_count = %d",
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
func (Bs *BlockScheduler) Halt() {
	// todo
}
