package blockstm

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

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

// BlockScheduler A deterministic parallel scheduler
type BlockScheduler struct {
	lock           sync.Mutex
	log            protocol.Logger
	chainConf      protocol.ChainConf
	storeHelper    conf.StoreHelper
	vmHelper       *deterministic.CommonVMHelper // Shared VM execution helper
	txRWSetMap     map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet  todo chainmaker用这个落库。
	txRWSetMapLock sync.Mutex                    // lock for txRWSetMap concurrent access todo 这个似乎没用上

	threadsNum int // 线程数量 comment：每个 thread 可以执行 execute 任务，也可以执行 validate 任务
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

		threadsNum: runtime.NumCPU(), // todo:确认是否为论文建议的配置数
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
	startTime := time.Now()
	Bs.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 创建调度器
	scheduler := NewScheduler(len(txBatch))
	mvMemory := NewMVMemory(len(txBatch))

	ctx, cancel := context.WithTimeout(context.Background(), ScheduleTimeout*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(Bs.threadsNum)
	for i := 0; i < Bs.threadsNum; i++ {
		t := NewThread(ctx, scheduler, mvMemory, txBatch, Bs.vmHelper, snapshot, block, i, Bs.log) // 创建每个线程器 comment：这里传入了全局txbatch，全局snapshot，全局scheduler， 全局mvMemory
		go func() {
			defer wg.Done()
			t.Run() // 让每个线程器跑起来
		}()
	}
	wg.Wait()

	// 从每笔交易最后一次incarnation的txSimContext中收集txRWSet，填充tx.Result和block.Txs
	for i, tx := range txBatch {
		ptr := mvMemory.lastTxSimContext[i].Load()
		if ptr == nil {
			panic(fmt.Sprintf("blockstm: txSimContext missing for tx[%d] %s", i, tx.Payload.TxId))
		}
		txSimCtx := *ptr
		txRWSet := txSimCtx.GetTxRWSet(true) // comment：runVmSuccess设为true
		if txRWSet != nil {
			Bs.txRWSetMap[tx.Payload.TxId] = txRWSet
		}
		tx.Result = txSimCtx.GetTxResult()
		block.Txs = append(block.Txs, tx)
	}

	totalTime := time.Since(startTime)
	tps := float64(len(block.Txs)) / totalTime.Seconds()
	Bs.log.Infof("BlockSTM schedule completed, total time=%v, total txs=%d, TPS=%.2f, blockheight=%d",
		totalTime, len(block.Txs), tps, block.Header.BlockHeight)

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
