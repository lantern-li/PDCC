/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package graph

import (
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

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20
	// DefaultBatchSizeMultiplier 默认批处理大小倍数（相对于CPU核心数）
	DefaultBatchSizeMultiplier = 10
)

// txExecInfo 存储交易执行的相关信息
type txExecInfo struct {
	tx                    *commonPb.Transaction
	txSimContext          protocol.TxSimContext
	txRWSet               *commonPb.TxRWSet
	txReadSet             []*commonPb.TxRead
	txWriteSetWithVersion []*commonPb.VersionedTxWrite
	originalIndex         int  // 原始索引，用于确定性排序
	rwSetCount            int  // 读写集总数量，用于重排序
	removed               bool // 是否因破环被移除
}

// GraphScheduler A deterministic parallel scheduler
type GraphScheduler struct {
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

// NewGraphSchedulerr creates a new Graph transaction scheduler
func NewGraphScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Infof("use the deterministic Graph scheduler")

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
	Gs.lock.Lock()
	defer Gs.lock.Unlock()
	defer Gs.vmHelper.ReleaseContractCache()
	Gs.log.Infof("Graph schedule start, block_number = %v, tx_count = %d, batchsize = %d", block.Header.BlockHeight, len(txBatch), Gs.batchSize) //动态调整后整理的ws.batchSize要改

	Gs.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 循环调度，直到 txBatch 为空或超时
	startTime := time.Now()
	timeoutDuration := time.Duration(ScheduleTimeout) * time.Second
	roundNum := 0

	for len(txBatch) > 0 {
		roundNum++

		// 检查超时
		if time.Since(startTime) > timeoutDuration {
			Gs.log.Warnf("Schedule timeout after %d rounds, remaining %d transactions", roundNum, len(txBatch))
			break
		}

		//1. 选择阶段：依据BatchSize从当前txBatch中选取前BatchSize个序号最小交易进行调度，txBatch为剩余交易池
		batchSize := Gs.batchSize
		if batchSize > len(txBatch) {
			batchSize = len(txBatch)
		}
		selectedTxs := txBatch[:batchSize:batchSize] // 容量也限制为 batchSize，不共享后面的容量空间。
		txBatch = txBatch[batchSize:]
		Gs.log.DebugDynamic(func() string {
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

				txSimContext, _, runTxSuccess := Gs.vmHelper.ExecuteTx(transaction, snapshot, block) // comment：都在相同的snapshot上执行

				txRWSet := txSimContext.GetTxRWSet(runTxSuccess)

				execInfos[idx] = txExecInfo{
					tx:           transaction,
					txSimContext: txSimContext,
					txRWSet:      txRWSet,
					txReadSet:    txRWSet.TxReads,
					//originalIndex: idx, Graph算法不需要这个。
					//rwSetCount:    len(txRWSet.TxReads) + len(txRWSet.TxWrites), Graph算法不需要这个。
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

		Gs.log.DebugDynamic(func() string {
			return fmt.Sprintf("[ExecutionStage] execute %d txs finished, total cost=%v", len(selectedTxs), time.Since(execStageStart))
		})

		// 3.写集合并阶段：将每笔交易的写集WS(TXi)进行合并，生成写集多版本总表MasterWS。
		writeSetMergingStart := time.Now()
		masterWS := buildMasterWriteSet(execInfos)
		Gs.log.DebugDynamic(func() string {
			return fmt.Sprintf("[writeSetMergingStage]: total cost=%v", time.Since(writeSetMergingStart))
		})

		// 4.构图阶段：基于读写依赖关系构建有向图
		graphBuildStart := time.Now()
		graph := buildDependencyGraph(execInfos, masterWS)
		Gs.log.DebugDynamic(func() string {
			edgeCount := 0
			for _, edges := range graph.Edges {
				edgeCount += len(edges)
			}
			return fmt.Sprintf("[graphBuildStage]: built graph with %d nodes and %d edges, total cost=%v",
				len(graph.Nodes), edgeCount, time.Since(graphBuildStart))
		})

		// 5.检测环阶段：使用DFS三色标记法检测有向图中的环
		cycleDetectStart := time.Now()

		hasCycle, cycleNodes := graph.DetectCycle()
		// comment：当hasCycle为true时，cycleNodes表示的是检测到的一条环路径上的节点集合。
		// 注意：cycleNodes返回的是 “一条发现的环路径（cycle path）”，而不是图中所有环。 算法可以是确定性的。先不纠结这一个环破环的优化。只用来确定是否有环。todo：后续考虑优化

		Gs.log.DebugDynamic(func() string {
			return fmt.Sprintf("[cycleDetectStage]: hasCycle=%v, cycleNodes=%v, total cost=%v", hasCycle, cycleNodes, time.Since(cycleDetectStart))
		})

		// 6. 破环阶段
		if hasCycle {
			cycleBreakStage := time.Now()
			// 拷贝一份图，用于后续破环操作，不影响原图
			graphCopy := graph.Copy()

			// 循环删除graphCopy中所有入度为0/出度为0的顶点，剩余节点必定在环上
			graphCopy.RemoveLeafNodes()

			/* comment：这时有如下3个结论必定成立：
			1、图中的每个强连通分量至少包含一个环。
			2、每个环必定被包含在某个强连通分量中。
			3、图中的每个强连通分量必定的大小必定大于等于2。（因为图中不允许自环的出现）
			*/

			// 使用 Tarjan 算法找出所有强连通分量
			sccs := graphCopy.FindSCCs()

			// 对每个 SCC 执行破环：循环选点删除 → RemoveLeafNodes，直到瓦解
			removedNodes := graphCopy.BreakCycles(sccs)

			// 标记被删除的交易
			for _, nodeIdx := range removedNodes {
				execInfos[nodeIdx].removed = true
			}

			Gs.log.DebugDynamic(func() string {
				return fmt.Sprintf("[cycleBreakStage]: removedNodes=%v, total cost=%v",
					removedNodes, time.Since(cycleBreakStage))
			})

		}

	}

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
