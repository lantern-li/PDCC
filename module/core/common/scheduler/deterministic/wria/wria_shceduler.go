/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package wria

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
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
)

var (
	// BatchSize 批处理大小，自动设置为 CPU 核心数的 4 倍
	BatchSize = runtime.NumCPU() * 4 // 这个应该设置为物理核心数还是逻辑核心数量？todo 似乎操作系统只能读到逻辑核心数量？
)

// txExecInfo 存储交易执行的相关信息
type txExecInfo struct {
	tx                    *commonPb.Transaction
	cost                  time.Duration
	txSimContext          protocol.TxSimContext
	txRWSet               *commonPb.TxRWSet
	txReadSet             []*commonPb.TxRead
	txWriteSetWithVersion []*commonPb.VersionedTxWrite
}

// WriaScheduler A deterministic parallel scheduler
type WriaScheduler struct {
	lock           sync.Mutex
	log            protocol.Logger
	chainConf      protocol.ChainConf
	storeHelper    conf.StoreHelper
	vmHelper       *deterministic.CommonVMHelper // Shared VM execution helper
	snapshotCache  sync.Map                      // key: string(Write.Key), value: *commonPb.VersionedTxWrite
	txRWSetMap     map[string]*commonPb.TxRWSet  // key: string(txId), value: *commonPb.TxRWSet
	txRWSetMapLock sync.Mutex                    // lock for txRWSetMap concurrent access
}

// NewWriaScheduler creates a new WRIA transaction scheduler
func NewWriaScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, storeHelper conf.StoreHelper, ac protocol.AccessControlProvider) protocol.TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.DebugDynamic(func() string {
		return "use the deterministic WRIA scheduler"
	})

	scheduler := &WriaScheduler{
		lock:        sync.Mutex{},
		log:         log,
		chainConf:   chainConf,
		storeHelper: storeHelper,
		txRWSetMap:  make(map[string]*commonPb.TxRWSet), // 初始化 txRWSetMap
		// snapshotCache sync.Map 不需要初始化
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
	ws.log.Infof("WRIA schedule start, block_number = %v, tx_count = %d, batchsize = %d", block.Header.BlockHeight, len(txBatch), BatchSize)

	ws.txRWSetMap = make(map[string]*commonPb.TxRWSet)
	block.Txs = nil // ← 添加这行！清空 block.Txs

	// 循环调度，直到 txBatch 为空或超时
	startTime := time.Now()
	timeoutDuration := time.Duration(ScheduleTimeout) * time.Second
	roundNum := 0

	for len(txBatch) > 0 {
		roundNum++

		// 检查超时
		if time.Since(startTime) > timeoutDuration {
			ws.log.Warnf("Schedule timeout after %d rounds, remaining %d transactions", roundNum, len(txBatch))
			break
		}

		//1. 选择阶段：依据BatchSize从当前txBatch中选取前BatchSize个序号最小交易进行调度，txBatch为剩余交易池
		batchSize := BatchSize
		if batchSize > len(txBatch) {
			batchSize = len(txBatch)
		}
		selectedTxs := txBatch[:batchSize:batchSize]
		txBatch = txBatch[batchSize:]
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("Round %d: selected %d transactions for scheduling, remainTxs=%d", roundNum, batchSize, len(txBatch))
		})

		// 2. 执行阶段：将selectedTxs并发地在当前相同的snapshot上执行。
		var wg sync.WaitGroup
		execInfos := make([]txExecInfo, len(selectedTxs))

		execStageStart := time.Now() // 记录开始时间

		for i, tx := range selectedTxs {
			wg.Add(1)
			go func(idx int, transaction *commonPb.Transaction) {
				defer wg.Done()

				start := time.Now()
				txSimContext, _, runTxSuccess := ws.vmHelper.ExecuteTx(transaction, snapshot, block)
				costTime := time.Since(start)

				txRWSet := txSimContext.GetTxRWSet(runTxSuccess)

				execInfos[idx] = txExecInfo{
					tx:           transaction,
					cost:         costTime,
					txSimContext: txSimContext,
					txRWSet:      txRWSet,
					txReadSet:    txRWSet.TxReads,
				}
			}(i, tx)
		}
		wg.Wait()

		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("[ExecutionStage] execute %d txs finished, total cost=%v", len(selectedTxs), time.Since(execStageStart))
		})

		// 3. 确定性重排序阶段：依据每笔交易的执行时间/读写集的大小，进行重排序，大的排在前面。
		// 执行时间长的排在前面
		//sort.Slice(execInfos, func(i, j int) bool {
		//	return execInfos[i].cost > execInfos[j].cost
		//}) // todo：后续设计非确定性的算法再排序

		// 4. 版本标记阶段：并发地将每笔交易的写集进行版本标记。
		versionMarkStart := time.Now()
		var versionWG sync.WaitGroup
		for txIndex, execInfo := range execInfos {
			versionWG.Add(1)
			go func(idx int, info txExecInfo) {
				defer versionWG.Done()

				txRWSet := info.txRWSet
				versionedWrites := make([]*commonPb.VersionedTxWrite, 0, len(txRWSet.TxWrites))

				for _, w := range txRWSet.TxWrites {
					versionedWrites = append(versionedWrites, &commonPb.VersionedTxWrite{
						Write:   w,
						Version: uint64(idx),
					})
				}

				execInfos[idx].txWriteSetWithVersion = versionedWrites
			}(txIndex, execInfo)
		}
		versionWG.Wait()
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("[versionMarkStage]: total cost=%v", time.Since(versionMarkStart))
		})

		// 5.写集合并阶段：并发地将每笔交易的写集WS(TXi)进行合并，生成写集多版本总表MasterWS。
		writeSetMergingStart := time.Now()

		type MasterWriteSet map[string][]*commonPb.VersionedTxWrite // string：string(Write.Key)
		masterWS := make(MasterWriteSet)

		// 并发处理：每个goroutine处理一个交易，生成局部map（避免锁竞争）
		localMaps := make([]MasterWriteSet, len(execInfos))
		var mergeWG sync.WaitGroup

		for i, execInfo := range execInfos {
			mergeWG.Add(1)
			go func(idx int, info txExecInfo) {
				defer mergeWG.Done()

				localMap := make(MasterWriteSet)
				for _, vw := range info.txWriteSetWithVersion {
					key := string(vw.Write.Key)
					localMap[key] = append(localMap[key], vw)
				}
				localMaps[idx] = localMap
			}(i, execInfo)
		}
		mergeWG.Wait()

		// 按序合并所有局部map到masterWS，保证升序（version按txIndex升序）
		for _, localMap := range localMaps {
			for key, vwList := range localMap {
				masterWS[key] = append(masterWS[key], vwList...)
			}
		}
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("[writeSetMergingStage]: total cost=%v", time.Since(writeSetMergingStart))
		})

		// 6.冲突检测阶段、提交阶段、再检查阶段
		//冲突检测RAW：依据MasterWS，对每笔交易的读集进行冲突检测，检测通过则立即启动协程应用写集，检测不通过则标记abort并记录冲突依赖。
		//提交：对于通过了RAW检测的交易，立即启动协程将其写集应用到snapshot cache中（不阻塞）。
		//再检查：当所有交易都完成了RAW检测后，立即触发再检查阶段。针对所有被abort的交易，串行地进行检查，
		//        检查规则：检查记录的冲突依赖前序交易是否都被abort了，如果是则挽救该交易；只要有一个前序交易没被abort，则继续abort。
		//等待：等待所有通过RAW检测的交易和被rechecking挽救的交易都完成写集应用。

		checkCommitAndRecheckingStart := time.Now()

		// abort 标记：初始均为 false。 true 表示该交易需要被abort
		abortFlags := make([]bool, len(execInfos))

		// 冲突依赖记录：记录每个被abort交易依赖的前序交易索引列表
		// conflictDeps[i] 表示交易i被abort是因为依赖了哪些前序交易
		conflictDeps := make([][]int, len(execInfos))

		// 完成 RAW 检测计数器
		var finishedCnt atomic.Int32

		// 提交阶段等待组，用于等待所有写集应用完成（包括通过RAW的和被rechecking挽救的）
		var applyWG sync.WaitGroup

		// rechecking完成信号
		recheckingDone := make(chan struct{})

		// 启动RAW冲突检测
		for i, execInfo := range execInfos {
			go func(txIndex int, info txExecInfo) {
				// 1. RAW冲突检测（完全并行），返回是否冲突以及冲突依赖的前序交易列表
				hasConflict, conflictingTxs := isRAWConflictWithDeps(txIndex, info.txReadSet, masterWS)

				if hasConflict {
					// 检测到 RAW 冲突，标记 abort 并记录冲突依赖
					abortFlags[txIndex] = true
					conflictDeps[txIndex] = conflictingTxs
				}

				// 2. 若未 abort，立即启动协程应用写集到当前SnapShot中
				if !abortFlags[txIndex] {
					applyWG.Add(1)
					go func(i int, info txExecInfo) {
						defer applyWG.Done()
						ws.applyWSToSnapshotCache(info.txRWSet, info.txWriteSetWithVersion)
					}(txIndex, info)
				}

				// 3. 计数并判断是否所有交易都已经完成RAW检测
				if finishedCnt.Add(1) == int32(len(execInfos)) {
					// 所有 TX 都完成检测，立即触发 rechecking
					go func() {
						ws.rechecking(execInfos, abortFlags, conflictDeps, &applyWG)
						close(recheckingDone)
					}()
				}
			}(i, execInfo)
		}

		// 等待rechecking完成
		<-recheckingDone

		// 等待所有成功交易完成写入 Cache（包括通过RAW的和被rechecking挽救的）
		applyWG.Wait()

		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("[checkCommitAndRecheckingStage]: total cost=%v", time.Since(checkCommitAndRecheckingStart))
		})

		// 7. 将 ws.snapshotCache 中的写集直接应用到 snapshot.writeTable 中，并清空 ws.snapshotCache
		// 这样下一批交易执行时，可以直接从 snapshot.writeTable 中读取，而不用从 DB 中读取
		applyWriteCacheToSnapshotStart := time.Now()
		appliedCount := ws.applySnapshotCacheToSnapshot(snapshot) // todo：后续考虑性能优化，不对snapshot进行适配
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("[applyWriteCacheToSnapshotStage]: total cost=%v, applynum:%d", time.Since(applyWriteCacheToSnapshotStart), appliedCount)
		})

		// 清空 snapshotCache
		//cacheSize := ws.getSnapshotCacheSize()
		ws.clearSnapshotCache()
		//ws.log.Infof("Cleared snapshotCache, was holding %d entries", cacheSize)

		// 8. 将上一批中最终被abort的交易放回剩余交易池txBatch，从剩余交易池中选取前BatchSize个交易进行下一轮调度
		// 收集被 abort 的交易
		abortedTxs := make([]*commonPb.Transaction, 0)
		committedTxs := 0
		for i := range execInfos {
			if abortFlags[i] {
				abortedTxs = append(abortedTxs, execInfos[i].tx) // 使用execInfos[i].tx以支持重排序
			} else {
				// 将这笔交易加进block.Txs
				execInfos[i].tx.Result = execInfos[i].txSimContext.GetTxResult() //注意这里
				block.Txs = append(block.Txs, execInfos[i].tx)                   // 已完成的交易按序加到block.Txs，这就是该调度产生的可序列化串行顺序
				committedTxs++                                                   // 非确定性调度中可以作为调度信心
			}
		}

		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("Round %d completed: committed=%d, aborted=%d", roundNum, committedTxs, len(abortedTxs))
		})

		// 将被 abort 的交易放回 txBatch 头部（prepend）
		if len(abortedTxs) > 0 {
			txBatch = append(abortedTxs, txBatch...)
			ws.log.DebugDynamic(func() string {
				return fmt.Sprintf("Prepended %d aborted transactions back to remaining txBatch, remaining txBatch size=%d", len(abortedTxs), len(txBatch))
			})
		}
	}

	totalTime := time.Since(startTime)
	tps := float64(len(block.Txs)) / totalTime.Seconds()
	ws.log.Infof("WRIA schedule completed after %d rounds, total time=%v, total txs=%d, TPS=%.2f, blockheight=%d",
		roundNum, totalTime, len(block.Txs), tps, block.Header.BlockHeight)

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
		for _, vw := range versionedWrites {
			if vw.Version < uint64(txIndex) {
				// 找到一个产生RAW冲突的前序交易
				txIdx := int(vw.Version)
				if !conflictMap[txIdx] {
					conflictMap[txIdx] = true
					conflictingTxs = append(conflictingTxs, txIdx) //注意这里要记录所有的冲突，后面rechecking要用。不能因为一个冲突就返回了。
				}
			}
		}
	}

	return len(conflictingTxs) > 0, conflictingTxs
}

// rechecking 串行检查所有被abort的交易，对于被挽救的交易，启动协程应用写集
// 该函数在所有交易完成RAW检测后被调用
// 检查规则：对于被abort的交易，检查其记录的冲突依赖前序交易是否都被abort了，如果是则挽救该交易
func (ws *WriaScheduler) rechecking(execInfos []txExecInfo, abortFlags []bool, conflictDeps [][]int, applyWG *sync.WaitGroup) {
	ws.log.DebugDynamic(func() string {
		return "Starting rechecking phase for aborted transactions"
	})

	rescuedCount := 0
	totalAborted := 0

	for txIndex, info := range execInfos {
		if !abortFlags[txIndex] {
			// 该交易未被abort，跳过
			continue
		}

		totalAborted++

		// 检查该交易是否被"冤枉"了
		// 直接使用预先记录的冲突依赖，无需重新遍历读集
		isRescued := ws.recheckTransaction(txIndex, conflictDeps[txIndex], abortFlags)

		if isRescued {
			// 交易被挽救，更新abort标记，并启动协程应用写集（不阻塞串行检查）
			abortFlags[txIndex] = false
			ws.log.DebugDynamic(func() string {
				return fmt.Sprintf("Transaction %d rescued during rechecking, had %d conflicting dependencies (all aborted)", txIndex, len(conflictDeps[txIndex]))
			})
			rescuedCount++
			applyWG.Add(1)
			go func(idx int, execInfo txExecInfo) {
				defer applyWG.Done()
				ws.applyWSToSnapshotCache(execInfo.txRWSet, execInfo.txWriteSetWithVersion)
			}(txIndex, info)
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

// applyWSToSnapshotCache 将写集应用到 snapshot cache 中，同时将原生读写集存储到全局 map
// 规则：只有当 record.version 在 cache 中是最大时，才应用到 cache —— CAS 在线合并方案
// 支持多协程并发安全地写入
// 参数：
//   - txRWSet: 原生的交易读写集
//   - txWritesWithVersion: 带版本的写集
func (ws *WriaScheduler) applyWSToSnapshotCache(txRWSet *commonPb.TxRWSet, txWritesWithVersion []*commonPb.VersionedTxWrite) {
	// 1. 先把这笔交易的读写集合加到全局的 map[string]*commonPb.TxRWSet 中
	if txRWSet != nil && txRWSet.TxId != "" {
		ws.txRWSetMapLock.Lock()
		ws.txRWSetMap[txRWSet.TxId] = txRWSet // 这里加锁保证安全
		ws.txRWSetMapLock.Unlock()
		ws.log.DebugDynamic(func() string {
			return fmt.Sprintf("Stored TxRWSet for txID=%s with %d reads and %d writes", txRWSet.TxId, len(txRWSet.TxReads), len(txRWSet.TxWrites))
		})
	}

	// 2. 将带版本的写集并发应用到 snapshot cache 中
	var wg sync.WaitGroup
	for _, vw := range txWritesWithVersion {
		wg.Add(1)
		go func(versionedWrite *commonPb.VersionedTxWrite) {
			defer wg.Done()

			key := string(versionedWrite.Write.Key)

			// 使用 CAS 循环来保证并发安全
			for {
				// 1. 尝试加载当前值
				if existing, ok := ws.snapshotCache.Load(key); ok {
					cached := existing.(*commonPb.VersionedTxWrite)

					// 2. 如果当前版本不是最大版本，直接跳过
					if versionedWrite.Version <= cached.Version { // todo :会等于吗？
						break
					}

					// 3. 当前版本更大，尝试使用 CAS 更新
					if ws.snapshotCache.CompareAndSwap(key, existing, &commonPb.VersionedTxWrite{
						Version: versionedWrite.Version,
						Write:   versionedWrite.Write,
					}) {
						// CAS 更新成功
						ws.log.DebugDynamic(func() string {
							return fmt.Sprintf("Updated cache for key=%s with version=%d (previous version=%d)", key, versionedWrite.Version, cached.Version)
						})
						break
					}
					// CAS 更新失败（其他协程修改了值），重试
					continue
				}

				// 4. key 不存在，尝试存储新值
				actual, exist := ws.snapshotCache.LoadOrStore(key, &commonPb.VersionedTxWrite{
					Version: versionedWrite.Version,
					Write:   versionedWrite.Write,
				})

				if !exist {
					// 成功存储新值
					ws.log.DebugDynamic(func() string {
						return fmt.Sprintf("Stored new cache entry for key=%s with version=%d", key, versionedWrite.Version)
					})
					break
				}

				// 5. LoadOrStore 期间其他协程已经存储了值，需要重新检查版本
				cached := actual.(*commonPb.VersionedTxWrite)
				if versionedWrite.Version < cached.Version {
					// 其他协程存储的版本更大，跳过
					break
				}
				// 其他协程存储的版本更小，继续循环尝试更新
			}
		}(vw)
	}
	wg.Wait()
}

// clearSnapshotCache 清空 snapshot cache
// 通常在一个批次调度完成后调用
func (ws *WriaScheduler) clearSnapshotCache() {
	ws.snapshotCache.Range(func(key, value interface{}) bool {
		ws.snapshotCache.Delete(key)
		return true
	})
	ws.log.Debug("Snapshot cache cleared")
}

// applySnapshotCacheToSnapshot 将 snapshotCache 中的写集应用到 snapshot.writeTable
// 这样下一批交易执行时，可以直接从 snapshot.writeTable 中读取，而不用从 DB 中读取
// 返回应用的写操作数量
func (ws *WriaScheduler) applySnapshotCacheToSnapshot(snap protocol.Snapshot) int {
	// 收集 snapshotCache 中的所有写操作 todo：考虑对每个写条目并发的写
	writes := make([]*commonPb.TxWrite, 0)
	ws.snapshotCache.Range(func(key, value interface{}) bool {
		versionedWrite, ok := value.(*commonPb.VersionedTxWrite)
		if !ok {
			ws.log.Warnf("Invalid value type in snapshotCache for key=%v", key)
			return true // 继续遍历
		}
		// 提取 TxWrite（不需要版本信息）
		writes = append(writes, versionedWrite.Write)
		return true
	})

	// 批量应用到 snapshot.writeTable（使用接口方法）
	if len(writes) > 0 {
		snap.ApplyWritesToWriteTable(writes)
	}

	return len(writes)
}

// getSnapshotCacheSize 获取 cache 中的条目数量
// 主要用于调试和监控
func (ws *WriaScheduler) getSnapshotCacheSize() int {
	count := 0
	ws.snapshotCache.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
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
