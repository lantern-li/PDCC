/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
// comment:读写冲突判断应该用return contractName + string(key)。之前之所以没错是因为压测是同一个合约，没体现出问题。先不管，因为后期要上EDCC了
package wria

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
)

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20

	// batchSize 动态调整参数（按“本次区块”的表现来调“下一个区块”的 batchSize）
	// 说明：只有当本次区块 tx 数足够大时，差值才具有代表性；否则跳过调整以避免抖动。
	batchAdjustMinTxCount   = 990
	HighBatchSizeMultiplier = 10
	LowBatchSizeMultiplier  = 1

	batchAdjustHighToLowDiff = 18
	batchAdjustLowToHighDiff = 10
)

// txExecInfo 存储交易执行的相关信息
type txExecInfo struct {
	tx                    *commonPb.Transaction
	txSimContext          protocol.TxSimContext
	txRWSet               *commonPb.TxRWSet
	txReadSet             []*commonPb.TxRead
	txWriteSetWithVersion []*commonPb.VersionedTxWrite
	originalIndex         int // 原始索引，用于确定性排序
	rwSetCount            int // 读写集总数量，用于重排序
}

// WriaScheduler A deterministic parallel scheduler
type WriaScheduler struct {
	lock        sync.Mutex
	log         protocol.Logger
	chainConf   protocol.ChainConf
	storeHelper conf.StoreHelper
	vmHelper    *deterministic.CommonVMHelper // Shared VM execution helper
	txRWSetMap  map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet  todo chainmaker的这个也要改
	batchSize   int                           // 批处理大小，从配置文件读取或使用默认值

	highBatchSize int // 高并发 batchSize
	lowBatchSize  int // 低并发 batchSize
}

// NewWriaScheduler creates a new WRIA transaction scheduler
func NewWriaScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Infof("use the deterministic PDCC scheduler")

	// 从配置文件读取 batch_size，如果未配置则使用默认值（CPU核心数 * 10）
	_ = int(chainConf.ChainConfig().Scheduler.GetBatchSize())

	scheduler := &WriaScheduler{
		log:           log,
		chainConf:     chainConf,
		storeHelper:   storeHelper,
		txRWSetMap:    make(map[string]*commonPb.TxRWSet),         // 初始化 txRWSetMap
		batchSize:     runtime.NumCPU() * HighBatchSizeMultiplier, //comment:启动时默认的batchSize
		highBatchSize: runtime.NumCPU() * HighBatchSizeMultiplier,
		lowBatchSize:  runtime.NumCPU() * LowBatchSizeMultiplier,
	}

	scheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return scheduler
}

// 说明：v2.3.5之后，交易执行时，如果从自己的写集中读取，那么这个读集不会被记录到最终txSimContext中的读集中

// Schedule schedules the transactions using WRIA algorithm.
func (ws *WriaScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	ws.lock.Lock()
	defer ws.lock.Unlock()
	defer ws.vmHelper.ReleaseContractCache()
	ws.log.Infof("WRIA schedule start, block_number = %v, tx_count = %d, batchsize = %d", block.Header.BlockHeight, len(txBatch), ws.batchSize) //动态调整后整理的ws.batchSize要改

	ws.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 循环调度，直到 txBatch 为空或超时
	startTime := time.Now()
	timeoutDuration := time.Duration(ScheduleTimeout) * time.Second
	roundNum := 0

	var (
		phase1Time, phase2Time, phase3Time, phase4Time, phase5Time time.Duration
		phase6Time, phase7Time, phase8Time, phase9Time             time.Duration
	)

	for len(txBatch) > 0 {
		roundNum++

		// 检查超时
		if time.Since(startTime) > timeoutDuration {
			ws.log.Warnf("Schedule timeout after %d rounds, remaining %d transactions", roundNum, len(txBatch))
			break
		}

		//1. Deterministic Selection：依据BatchSize从当前txBatch中选取前BatchSize个序号最小交易进行调度，txBatch为剩余交易池
		t := time.Now()
		batchSize := ws.batchSize
		if batchSize > len(txBatch) {
			batchSize = len(txBatch)
		}
		selectedTxs := make([]*commonPb.Transaction, batchSize)
		copy(selectedTxs, txBatch[:batchSize]) // copy后，selectedTxs是新的底层数组
		txBatch = txBatch[batchSize:]
		phase1Time += time.Since(t)

		// 2. Execution：将selectedTxs并发地在当前相同的snapshot上执行。
		t = time.Now()
		var wg sync.WaitGroup
		execInfos := make([]txExecInfo, len(selectedTxs))
		for i, tx := range selectedTxs {
			wg.Add(1)
			go func(idx int, transaction *commonPb.Transaction) {
				defer wg.Done()

				txSimContext, _, runTxSuccess := ws.vmHelper.ExecuteTx(transaction, snapshot, block)

				txRWSet := txSimContext.GetTxRWSet(runTxSuccess)

				execInfos[idx] = txExecInfo{
					tx:            transaction,
					txSimContext:  txSimContext,
					txRWSet:       txRWSet,
					txReadSet:     txRWSet.TxReads,
					originalIndex: idx,
					rwSetCount:    len(txRWSet.TxReads) + len(txRWSet.TxWrites),
				}
			}(i, tx)
		}
		wg.Wait()
		phase2Time += time.Since(t)

		// 3. Deterministic Reordering：依据每笔交易读写集的数量，进行重排序。每笔交易读写集的数量越多，越靠前。
		// 使用 sort.SliceStable 保证稳定排序（相同 rwSetCount 时保持原始顺序）
		t = time.Now()
		sort.SliceStable(execInfos, func(i, j int) bool {
			// 首先按 rwSetCount 降序排序（数量多的靠前）
			if execInfos[i].rwSetCount != execInfos[j].rwSetCount {
				return execInfos[i].rwSetCount > execInfos[j].rwSetCount
			}
			// rwSetCount 相同时，按原始索引升序排序（保证确定性）
			return execInfos[i].originalIndex < execInfos[j].originalIndex
		})
		phase3Time += time.Since(t)

		// 4. Version Tagging：对每笔交易的写集进行版本标记（需在重排序之后进行版本标记）。
		// 这里的处理是轻任务（遍历写集并附加版本），使用串行方式通常更高效且更稳定。
		t = time.Now()
		for txIndex := range execInfos {
			txRWSet := execInfos[txIndex].txRWSet
			txWrites := txRWSet.TxWrites

			versionedWrites := make([]*commonPb.VersionedTxWrite, len(txWrites))
			for i, w := range txWrites { // 这里不用append，直接按索引写入。更高效的写法。
				versionedWrites[i] = &commonPb.VersionedTxWrite{
					Write:   w,
					Version: uint64(txIndex),
				}
			}
			execInfos[txIndex].txWriteSetWithVersion = versionedWrites
		}
		phase4Time += time.Since(t)

		// 5.Write-Set Merging：将每笔交易的写集WS(TXi)进行合并，生成写集多版本总表MasterWS。
		t = time.Now()
		type MasterWriteSet map[string][]*commonPb.VersionedTxWrite // string：string(Write.Key)
		masterWS := make(MasterWriteSet)

		// 直接按序合并，一笔交易对同一个key只有一个写入，无需并发构建中间localMap
		for _, execInfo := range execInfos {
			for _, vw := range execInfo.txWriteSetWithVersion {
				key := string(vw.Write.Key)
				masterWS[key] = append(masterWS[key], vw) // comment：注意这里versionedWrites 已按 Version 升序
			}
		}
		phase5Time += time.Since(t)

		// 6. Conflict Detection：依据MasterWS，对每笔交易的读集进行冲突检测，检测不通过则标记abort并记录冲突依赖。todo：这里先串行吧
		// abort 标记：初始均为 false。 true 表示该交易需要被abort
		t = time.Now()
		abortFlags := make([]bool, len(execInfos))
		// 冲突依赖记录：记录每个被abort交易依赖的前序交易索引列表
		// conflictDeps[i] 表示交易i被abort是因为依赖了哪些前序交易
		conflictDeps := make([][]int, len(execInfos))
		for txIndex := range execInfos {
			info := execInfos[txIndex]
			hasConflict, conflictingTxs := isRAWConflictWithDeps(txIndex, info.txReadSet, masterWS)
			if hasConflict {
				abortFlags[txIndex] = true
				conflictDeps[txIndex] = conflictingTxs
			}
		}
		phase6Time += time.Since(t)

		// 7. Re-validation：检查记录的冲突依赖前序交易是否都被abort了，如果是则挽救该交易；只要有一个前序交易没被abort，则继续abort。
		//t = time.Now()
		//ws.rechecking(execInfos, abortFlags, conflictDeps)
		//phase7Time += time.Since(t)

		// 8. Commit：合并写集后一次性应用到 snapshot.writeTable
		t = time.Now()
		mergedWrites := make(map[string]*commonPb.TxWrite) // key -> 最终要应用的 TxWrite
		for txIndex, execInfo := range execInfos {
			if abortFlags[txIndex] {
				continue
			}
			ws.txRWSetMap[execInfo.txRWSet.TxId] = execInfo.txRWSet
			for _, w := range execInfo.txRWSet.TxWrites {
				// txIndex 按升序遍历，后覆盖即保留序号最大的写
				mergedWrites[string(w.Key)] = w
			}
		}
		if len(mergedWrites) > 0 {
			writes := make([]*commonPb.TxWrite, 0, len(mergedWrites))
			for _, w := range mergedWrites {
				writes = append(writes, w)
			}
			snapshot.ApplyWritesToWriteTable(writes)
		}
		phase8Time += time.Since(t)

		// 9. Transaction Reset：将上一批中最终被abort的交易放回剩余交易池txBatch，从剩余交易池中选取前BatchSize个交易进行下一轮调度
		t = time.Now()
		abortedTxs := make([]*commonPb.Transaction, 0)
		committedTxs := 0
		for i := range execInfos {
			if abortFlags[i] {
				abortedTxs = append(abortedTxs, execInfos[i].tx)
			} else {
				execInfos[i].tx.Result = execInfos[i].txSimContext.GetTxResult()
				block.Txs = append(block.Txs, execInfos[i].tx) // comment：可串行顺序
				committedTxs++                                 // comment：非确定性调度中可以作为调度信息
			}
		}
		// 将被 abort 的交易放回 txBatch 头部（prepend）
		if len(abortedTxs) > 0 {
			txBatch = append(abortedTxs, txBatch...)
		}
		phase9Time += time.Since(t)
	}
	// 以区块为单位进行batchsize动态调整
	ws.adjustBatchSize(len(block.Txs), roundNum)

	totalTime := time.Since(startTime)
	tps := float64(len(block.Txs)) / totalTime.Seconds()
	ws.log.Infof("WRIA schedule completed after %d rounds, total time=%v, total txs=%d, TPS=%.2f, blockheight=%d",
		roundNum, totalTime, len(block.Txs), tps, block.Header.BlockHeight)
	ws.log.Infof("WRIA phase time: phase1(selection)=%v phase2(execution)=%v phase3(reordering)=%v phase4(versionTagging)=%v phase5(merging)=%v phase6(conflictDetection)=%v phase7(revalidation)=%v phase8(commit)=%v phase9(txReset)=%v",
		phase1Time, phase2Time, phase3Time, phase4Time, phase5Time, phase6Time, phase7Time, phase8Time, phase9Time)

	return ws.txRWSetMap, nil, nil
}

// isRAWConflictWithDeps 检查交易是否存在RAW冲突，并返回导致冲突的前序交易列表
// 参数：
//   - txIndex: 当前交易的索引（版本号）
//   - readSet: 当前交易的读集
//   - masterWS: 写集多版本总表
//
// 返回：
//   - hasConflict: 是否存在RAW冲突
//   - conflictingTxs: 导致冲突的前序交易索引列表（如果无冲突则为空切片）
func isRAWConflictWithDeps(txIndex int, readSet []*commonPb.TxRead, masterWS map[string][]*commonPb.VersionedTxWrite) (bool, []int) {
	conflictingTxs := make([]int, 0)
	conflictMap := make(map[int]bool) // 用于去重

	for _, r := range readSet {
		key := string(r.Key)

		versionedWrites, exists := masterWS[key]
		if !exists {
			continue
		}

		// versionedWrites 已按 Version 升序
		// 找出所有版本号小于当前交易的写操作（即前序交易）
		for _, vw := range versionedWrites { // 这里从最小的version开始找
			if vw.Version >= uint64(txIndex) {
				break
			}

			// 找到一个产生RAW冲突的前序交易
			txIdx := int(vw.Version)
			if !conflictMap[txIdx] {
				conflictMap[txIdx] = true
				conflictingTxs = append(conflictingTxs, txIdx) // comment：注意这里要记录所有的冲突，后面rechecking要用。不能因为一个冲突就返回了。
			}
		}
	}

	return len(conflictingTxs) > 0, conflictingTxs
}

// rechecking 串行检查被abort的交易，检查其记录的冲突依赖前序交易是否都被abort了，如果是则挽救该交易
func (ws *WriaScheduler) rechecking(execInfos []txExecInfo, abortFlags []bool, conflictDeps [][]int) {
	ws.log.DebugDynamic(func() string {
		return "Starting rechecking phase for aborted transactions"
	})

	rescuedCount := 0
	totalAborted := 0

	for txIndex := range execInfos {
		if !abortFlags[txIndex] {
			// 该交易未被abort，跳过
			continue
		}

		totalAborted++

		// 检查该交易是否被"冤枉"了
		// 直接使用预先记录的冲突依赖，无需重新遍历读集
		isRescued := ws.recheckTransaction(txIndex, conflictDeps[txIndex], abortFlags)

		if isRescued {
			// 交易被挽救，更新abort标记（写集应用延后到 rechecking 完成后统一执行）
			abortFlags[txIndex] = false
			ws.log.DebugDynamic(func() string {
				return fmt.Sprintf("Transaction %d rescued during rechecking, had %d conflicting dependencies (all aborted)", txIndex, len(conflictDeps[txIndex]))
			})
			rescuedCount++
		} else {
			ws.log.DebugDynamic(func() string {
				return fmt.Sprintf("Transaction %d remains aborted, at least one of %d conflicting dependencies was committed", txIndex, len(conflictDeps[txIndex]))
			})
		}
	}

	ws.log.DebugDynamic(func() string {
		return fmt.Sprintf("Rechecking phase completed: rescued %d out of %d aborted transactions", rescuedCount, totalAborted)
	})
}

// recheckTransaction 对单个被abort的交易进行再检查
// 检查逻辑：检查冲突依赖的前序交易是否都被abort了
// 参数：
//   - txIndex: 当前交易的索引（版本号）
//   - conflictingTxs: 导致该交易被abort的前序交易索引列表（在RAW检测阶段记录的）
//   - abortFlags: 所有交易的abort标记数组
//
// 返回：true 表示挽救该交易，false 表示继续abort
func (ws *WriaScheduler) recheckTransaction(txIndex int, conflictingTxs []int, abortFlags []bool) bool {
	// 检查所有冲突依赖的前序交易
	for _, conflictTxIdx := range conflictingTxs {
		if !abortFlags[conflictTxIdx] {
			// 前序交易没有被abort（即被提交了），当前交易无法被挽救
			ws.log.DebugDynamic(func() string {
				return fmt.Sprintf("Transaction %d cannot be rescued: depends on committed transaction %d", txIndex, conflictTxIdx)
			})
			return false
		}
	}

	// 所有产生RAW冲突的前序交易都被abort了，当前交易可以被挽救
	ws.log.DebugDynamic(func() string {
		return fmt.Sprintf("Transaction %d can be rescued: all %d conflicting predecessors were aborted", txIndex, len(conflictingTxs))
	})
	return true
}

// commit 阶段已直接合并写集并应用到 snapshot.writeTable，无需 snapshot cache

// adjustBatchSize 根据本次区块的实际回滚轮次动态调整 batchSize（用于下一个区块）。
// 基准轮次 = ceil(blockTxCount / batchSize)（无冲突时的理论最小轮次）
// 差值 = 实际轮次 - 基准轮次
// 规则：
//   - 当前 batchSize=high：diff >= batchAdjustHighToLowDiff 时降到 low
//   - 当前 batchSize=low：diff < batchAdjustLowToHighDiff 时恢复到 high（滞回避免抖动）
func (ws *WriaScheduler) adjustBatchSize(blockTxCount, actualRoundNum int) {

	// 用“本次区块”的数据来决定“下一个区块”用什么 batchSize
	if blockTxCount < batchAdjustMinTxCount {
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("WRIA batchSize adjust skipped (txCount=%d, minTx=%d)", blockTxCount, batchAdjustMinTxCount)
		})
		return
	}

	baseRound := (blockTxCount + ws.batchSize - 1) / ws.batchSize // todo：确认向上取整

	diff := actualRoundNum - baseRound

	newBatchSize := ws.batchSize
	if ws.batchSize == ws.highBatchSize {
		if diff >= batchAdjustHighToLowDiff {
			newBatchSize = ws.lowBatchSize
		}
	} else if ws.batchSize == ws.lowBatchSize {
		if diff < batchAdjustLowToHighDiff {
			newBatchSize = ws.highBatchSize
		}
	}

	if newBatchSize != ws.batchSize {
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("WRIA batchSize adjusted: %d -> %d (txCount=%d, batchUsed=%d, rounds=%d, baseRounds=%d, diff=%d)",
				ws.batchSize, newBatchSize, blockTxCount, ws.batchSize, actualRoundNum, baseRound, diff)
		})
		ws.batchSize = newBatchSize
	}
}

// SimulateWithDag simulates the execution of transactions using the DAG in the block.
func (ws *WriaScheduler) SimulateWithDag(block *commonPb.Block, snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	ws.lock.Lock()
	defer ws.lock.Unlock()
	defer ws.vmHelper.ReleaseContractCache()

	ws.log.Infof("WRIA simulate with DAG start, block_number = %v, tx_count = %d",
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
func (ws *WriaScheduler) Halt() {
	// todo
}
