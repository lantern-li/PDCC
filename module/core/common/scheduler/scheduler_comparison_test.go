/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/wria"
	crypto2 "chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/pb-go/v2/consensus"
)

// ConflictScenario 定义冲突场景类型
type ConflictScenario int

const (
	ConflictScenarioNone  ConflictScenario = iota // 无冲突
	ConflictScenarioLow                           // 低冲突 (10%)
	ConflictScenarioHigh                          // 高冲突 (50%)
	ConflictScenarioChain                         // 链式依赖
)

// ComparisonMetrics 对比测试的性能指标
type ComparisonMetrics struct {
	SchedulerName string
	TotalTxs      int
	TotalTime     time.Duration
	Throughput    float64
	TPS           float64
}

// TestSchedulerComparison_ConflictRates 测试不同冲突率下两个调度器的性能对比
// 对比 WRIA 调度器（确定性）和非确定性调度器在不同冲突场景下的表现
func TestSchedulerComparison_ConflictRates(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  调度器性能对比测试：不同冲突率场景")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	txCount := 5000
	scenarios := []ConflictScenario{
		ConflictScenarioNone,
		ConflictScenarioLow,
		ConflictScenarioHigh,
		ConflictScenarioChain,
	}
	// 只包含启用的场景名称，与 scenarios 数组长度保持一致
	scenarioNames := []string{
		"无冲突 (0%)",
		"低冲突 (10%)",
		"高冲突 (50%)",
		"链式依赖 (100%)",
	}

	fmt.Printf("测试配置：\n")
	fmt.Printf("  交易数量: %d\n", txCount)
	fmt.Printf("  CPU核数: %d\n", runtime.NumCPU())
	fmt.Printf("  WRIA BatchSize: %d\n\n", runtime.NumCPU()*wria.DefaultBatchSizeMultiplier)

	// 存储结果用于最终对比
	wriaResults := make([]ComparisonMetrics, len(scenarios))
	ndResults := make([]ComparisonMetrics, len(scenarios))

	for i, scenario := range scenarios {
		fmt.Printf("【场景 %d/%d】%s\n", i+1, len(scenarios), scenarioNames[i])
		fmt.Println(strings.Repeat("-", 80))

		// 测试 WRIA 调度器
		wriaMetrics := runSchedulerComparisonTest(t, txCount, scenario, true)
		wriaResults[i] = wriaMetrics

		// 测试非确定性调度器
		ndMetrics := runSchedulerComparisonTest(t, txCount, scenario, false)
		ndResults[i] = ndMetrics

		// 打印对比结果
		printComparisonRow(scenarioNames[i], wriaMetrics, ndMetrics)
		fmt.Println()
	}

	// 打印汇总对比表
	printComparisonSummary(scenarioNames, wriaResults, ndResults)
}

// TestSchedulerComparison_TxCounts 测试不同交易数量下两个调度器的性能对比
func TestSchedulerComparison_TxCounts(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  调度器性能对比测试：不同交易数量 (低冲突场景)")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	scenario := ConflictScenarioLow
	txCounts := []int{50}

	fmt.Printf("测试配置：\n")
	fmt.Printf("  冲突场景: 低冲突 (10%%)\n")
	fmt.Printf("  CPU核数: %d\n", runtime.NumCPU())
	fmt.Printf("  WRIA BatchSize: %d\n\n", runtime.NumCPU()*wria.DefaultBatchSizeMultiplier)

	wriaResults := make([]ComparisonMetrics, len(txCounts))
	ndResults := make([]ComparisonMetrics, len(txCounts))

	fmt.Printf("%-10s | %-20s | %-20s | %-15s\n", "交易数", "WRIA吞吐量(tx/s)", "非确定性吞吐量(tx/s)", "性能比")
	fmt.Println(strings.Repeat("-", 80))

	for i, txCount := range txCounts {
		// 测试 WRIA 调度器
		wriaMetrics := runSchedulerComparisonTest(t, txCount, scenario, true)
		wriaResults[i] = wriaMetrics

		// 测试非确定性调度器
		ndMetrics := runSchedulerComparisonTest(t, txCount, scenario, false)
		ndResults[i] = ndMetrics

		// 打印单行对比
		ratio := wriaMetrics.Throughput / ndMetrics.Throughput
		ratioStr := fmt.Sprintf("%.2fx", ratio)
		if ratio < 1 {
			ratioStr = fmt.Sprintf("%.2fx ↓", ratio)
		} else if ratio > 1 {
			ratioStr = fmt.Sprintf("%.2fx ↑", ratio)
		} else {
			ratioStr = "1.00x ="
		}

		fmt.Printf("%-10d | %18.2f | %20.2f | %15s\n",
			txCount, wriaMetrics.Throughput, ndMetrics.Throughput, ratioStr)
	}

	// 分析可扩展性
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("可扩展性分析：")
	fmt.Println(strings.Repeat("-", 80))

	analyzeScalability("WRIA调度器", txCounts, wriaResults)
	fmt.Println()
	analyzeScalability("非确定性调度器", txCounts, ndResults)
}

// TestSchedulerComparison_HighLoad 测试高负载场景 (5000 笔交易)
func TestSchedulerComparison_HighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过高负载测试 (使用 -short 标志)")
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  调度器性能对比测试：高负载场景 (5000 笔交易)")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	txCount := 5000
	scenarios := []ConflictScenario{
		ConflictScenarioNone,
		ConflictScenarioLow,
		ConflictScenarioHigh,
	}
	scenarioNames := []string{
		"无冲突",
		"低冲突",
		"高冲突",
	}

	fmt.Printf("%-12s | %-18s | %-18s | %-12s\n", "场景", "WRIA(tx/s)", "非确定性(tx/s)", "性能比")
	fmt.Println(strings.Repeat("-", 80))

	for i, scenario := range scenarios {
		wriaMetrics := runSchedulerComparisonTest(t, txCount, scenario, true)
		ndMetrics := runSchedulerComparisonTest(t, txCount, scenario, false)

		ratio := wriaMetrics.Throughput / ndMetrics.Throughput
		fmt.Printf("%-12s | %16.2f | %18.2f | %11.2fx\n",
			scenarioNames[i], wriaMetrics.Throughput, ndMetrics.Throughput, ratio)
	}
}

// ========== 辅助函数 ==========

// runSchedulerComparisonTest 运行单个调度器的测试并返回指标
func runSchedulerComparisonTest(t *testing.T, txCount int, scenario ConflictScenario, useWria bool) ComparisonMetrics {
	// 配置
	localconf.ChainMakerConfig.NodeConfig.PrivKeyFile = TestPrivKeyFile
	localconf.ChainMakerConfig.NodeConfig.CertFile = TestCertFile
	localconf.ChainMakerConfig.NodeConfig.PrivKeyPassword = "11111111"

	// 准备环境
	ctl := gomock.NewController(t)
	snapshot, scheduler, block := prepareSchedulerEnvironment(t, ctl, txCount, scenario, useWria)

	// 生成交易
	txBatch := generateTransactions(txCount, scenario)

	// 记录开始时间
	startTime := time.Now()

	// 执行调度
	_, _, err := scheduler.Schedule(block, txBatch, snapshot)

	// 记录总耗时
	totalTime := time.Since(startTime)

	require.NoError(t, err)

	// 计算指标
	schedulerName := "非确定性调度器"
	if useWria {
		schedulerName = "WRIA调度器"
	}

	return ComparisonMetrics{
		SchedulerName: schedulerName,
		TotalTxs:      txCount,
		TotalTime:     totalTime,
		Throughput:    float64(txCount) / totalTime.Seconds(),
		TPS:           float64(txCount) / totalTime.Seconds(),
	}
}

// prepareSchedulerEnvironment 准备调度器测试环境
func prepareSchedulerEnvironment(t *testing.T, ctl *gomock.Controller, txCount int, scenario ConflictScenario, useWria bool) (
	protocol.Snapshot, protocol.TxScheduler, *commonPb.Block) {

	// 创建 mocks
	snapshot := mock.NewMockSnapshot(ctl)
	vmMgr := mock.NewMockVmManager(ctl)
	chainConf := mock.NewMockChainConf(ctl)
	storeHelper := mock.NewMockStoreHelper(ctl)
	ac := mock.NewMockAccessControlProvider(ctl)
	ledgerCache := mock.NewMockLedgerCache(ctl)

	// 配置 ChainConf
	chainConfig := &configpb.ChainConfig{
		ChainId: "chain1",
		Crypto: &configpb.CryptoConfig{
			Hash: crypto2.CRYPTO_ALGO_SHA256,
		},
		Vm: &configpb.Vm{
			AddrType: configpb.AddrType_CHAINMAKER,
		},
		Consensus: &configpb.ConsensusConfig{
			Type: consensus.ConsensusType_TBFT,
		},
		Core: &configpb.CoreConfig{
			EnableConflictsBitWindow: true,
		},
		Contract: &configpb.ContractConfig{
			EnableSqlSupport: false,
		},
		AuthType: protocol.Identity,
	}
	chainConf.EXPECT().ChainConfig().Return(chainConfig).AnyTimes()

	// 配置 StoreHelper
	storeHelper.EXPECT().GetPoolCapacity().Return(runtime.NumCPU() * 4).AnyTimes()

	// 配置 LedgerCache（非确定性调度器需要）
	ledgerCache.EXPECT().CurrentHeight().Return(uint64(99), nil).AnyTimes()

	// 配置 VmMgr - 根据场景生成不同的读写集
	vmMgr.EXPECT().RunContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(contract *commonPb.Contract, method string, byteCode []byte, parameters map[string][]byte,
			txSimContext protocol.TxSimContext, gasUsed uint64, refTxType commonPb.TxType) (
			*commonPb.ContractResult, protocol.ExecOrderTxType, commonPb.TxStatusCode) {

			// 根据场景生成读写集
			generateReadWriteSet(txSimContext, scenario)

			return &commonPb.ContractResult{
				Code:    0,
				Result:  []byte("success"),
				Message: "OK",
			}, protocol.ExecOrderTxTypeNormal, commonPb.TxStatusCode_SUCCESS
		}).AnyTimes()

	vmMgr.EXPECT().BeforeSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()
	vmMgr.EXPECT().AfterSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()

	// 配置 Snapshot
	setupSnapshotMocksForComparison(snapshot, txCount, chainConfig, ctl)

	// 配置 AccessControl
	ac.EXPECT().CreatePrincipal(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// 创建调度器
	var scheduler protocol.TxScheduler
	if useWria {
		scheduler = wria.NewWriaScheduler(vmMgr, chainConf, storeHelper, ac)
	} else {
		// 创建非确定性调度器
		metricCounter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_contract_invoke",
			},
			[]string{"chainId", "contractName", "runtimeType", "isSuccess"},
		)
		scheduler = NewTxSchedulerForTest(vmMgr, chainConf, storeHelper, ledgerCache, ac, nil, metricCounter)
	}

	// 创建区块
	block := &commonPb.Block{
		Header: &commonPb.BlockHeader{
			ChainId:      "chain1",
			BlockHeight:  100,
			BlockVersion: 2030400,
		},
		Dag: &commonPb.DAG{},
	}

	return snapshot, scheduler, block
}

// setupSnapshotMocksForComparison 配置 Snapshot 的 mock 行为（用于对比测试）
func setupSnapshotMocksForComparison(snapshot *mock.MockSnapshot, txCount int, chainConfig *configpb.ChainConfig, ctl *gomock.Controller) {
	// 使用线程安全的 map 存储写入的数据
	var dataLock sync.RWMutex
	dataStore := make(map[string][]byte)

	// 存储交易读写集
	var txRWSetTableLock sync.Mutex
	txRWSetTable := make([]*commonPb.TxRWSet, 0, txCount)

	// 存储交易表
	var txTableLock sync.Mutex
	txTable := make([]*commonPb.Transaction, 0, txCount)

	snapshot.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txExecSeq int, contractName string, key []byte) ([]byte, error) {
			dataLock.RLock()
			defer dataLock.RUnlock()
			if value, exists := dataStore[string(key)]; exists {
				return value, nil
			}
			return nil, nil
		}).AnyTimes()

	snapshot.EXPECT().ApplyWritesToWriteTable(gomock.Any()).
		DoAndReturn(func(writes []*commonPb.TxWrite) {
			dataLock.Lock()
			defer dataLock.Unlock()
			for _, w := range writes {
				dataStore[string(w.Key)] = w.Value
			}
		}).AnyTimes()

	snapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType, runVmSuccess bool, checkNeed bool) (bool, int) {
			// 非确定性调度器使用此方法
			tx := txSimContext.GetTx()
			txRWSet := txSimContext.GetTxRWSet(runVmSuccess)

			txTableLock.Lock()
			txTable = append(txTable, tx)
			currentSize := len(txTable)
			txTableLock.Unlock()

			txRWSetTableLock.Lock()
			txRWSetTable = append(txRWSetTable, txRWSet)
			txRWSetTableLock.Unlock()

			// 应用写集
			for _, w := range txRWSet.TxWrites {
				dataLock.Lock()
				dataStore[string(w.Key)] = w.Value
				dataLock.Unlock()
			}

			// 返回已应用的交易总数，而不是写集的大小
			return true, currentSize
		}).AnyTimes()

	// 添加 signer 相关的mock（避免 reflect panic）
	snapshot.EXPECT().GetPreSnapshot().Return(nil).AnyTimes()

	snapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	snapshot.EXPECT().Seal().DoAndReturn(func() {
		// DAG 构建时调用
	}).AnyTimes()

	snapshot.EXPECT().BuildDAG(gomock.Any(), gomock.Any()).DoAndReturn(func(isSql bool, txRWSetTable []*commonPb.TxRWSet) *commonPb.DAG {
		// 返回一个简单的 DAG
		txTableLock.Lock()
		defer txTableLock.Unlock()
		vertexes := make([]*commonPb.DAG_Neighbor, len(txTable))
		for i := range txTable {
			vertexes[i] = &commonPb.DAG_Neighbor{
				Neighbors: []uint32{}, // 简化：无依赖关系
			}
		}
		return &commonPb.DAG{Vertexes: vertexes}
	}).AnyTimes()

	snapshot.EXPECT().GetTxTable().DoAndReturn(func() []*commonPb.Transaction {
		txTableLock.Lock()
		defer txTableLock.Unlock()
		return txTable
	}).AnyTimes()

	snapshot.EXPECT().GetTxRWSetTable().DoAndReturn(func() []*commonPb.TxRWSet {
		txRWSetTableLock.Lock()
		defer txRWSetTableLock.Unlock()
		return txRWSetTable
	}).AnyTimes()

	snapshot.EXPECT().GetSnapshotSize().Return(0).AnyTimes()
	snapshot.EXPECT().GetLastChainConfig().Return(chainConfig).AnyTimes()
	snapshot.EXPECT().GetTxResultMap().Return(make(map[string]*commonPb.Result)).AnyTimes()
	snapshot.EXPECT().GetSpecialTxTable().Return([]*commonPb.Transaction{}).AnyTimes()
	snapshot.EXPECT().GetBlockFingerprint().Return("100").AnyTimes()

	blockChainStore := mock.NewMockBlockchainStore(ctl)
	blockChainStore.EXPECT().GetContractByName(gomock.Any()).Return(&commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}, nil).AnyTimes()
	blockChainStore.EXPECT().GetContractBytecode(gomock.Any()).Return([]byte{}, nil).AnyTimes()
	snapshot.EXPECT().GetBlockchainStore().Return(blockChainStore).AnyTimes()
}

// generateTransactions 生成测试交易
func generateTransactions(count int, scenario ConflictScenario) []*commonPb.Transaction {
	txs := make([]*commonPb.Transaction, count)
	contractId := &commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	for i := 0; i < count; i++ {
		txId := fmt.Sprintf("tx%032d", i)
		txs[i] = newTxForComparison(txId, contractId, make(map[string]string))
	}

	return txs
}

// generateReadWriteSet 根据场景生成读写集
func generateReadWriteSet(txSimContext protocol.TxSimContext, scenario ConflictScenario) {
	tx := txSimContext.GetTx()
	txId := tx.Payload.TxId
	contractName := tx.Payload.ContractName

	// 从 txId 中提取序号
	var txIndex int
	fmt.Sscanf(txId, "tx%d", &txIndex)

	switch scenario {
	case ConflictScenarioNone:
		// 无冲突：每个交易写不同的 key
		key := fmt.Sprintf("key_%d", txIndex)
		txSimContext.Put(contractName, []byte(key), []byte(fmt.Sprintf("value_%d", txIndex)))

	case ConflictScenarioLow:
		// 低冲突：10% 的交易访问共享 key
		if txIndex%10 == 0 {
			// 读写共享 key
			txSimContext.Get(contractName, []byte("shared_key"))
			txSimContext.Put(contractName, []byte("shared_key"), []byte(fmt.Sprintf("value_%d", txIndex)))
		} else {
			// 独立 key
			key := fmt.Sprintf("key_%d", txIndex)
			txSimContext.Put(contractName, []byte(key), []byte(fmt.Sprintf("value_%d", txIndex)))
		}

	case ConflictScenarioHigh:
		// 高冲突：50% 的交易访问共享 key
		if txIndex%2 == 0 {
			txSimContext.Get(contractName, []byte("shared_key"))
			txSimContext.Put(contractName, []byte("shared_key"), []byte(fmt.Sprintf("value_%d", txIndex)))
		} else {
			key := fmt.Sprintf("key_%d", txIndex)
			txSimContext.Put(contractName, []byte(key), []byte(fmt.Sprintf("value_%d", txIndex)))
		}

	case ConflictScenarioChain:
		// 链式依赖：每个交易读前一个交易写的 key
		if txIndex > 0 {
			prevKey := fmt.Sprintf("key_%d", txIndex-1)
			txSimContext.Get(contractName, []byte(prevKey))
		}
		currentKey := fmt.Sprintf("key_%d", txIndex)
		txSimContext.Put(contractName, []byte(currentKey), []byte(fmt.Sprintf("value_%d", txIndex)))
	}
}

// newTxForComparison 创建测试交易
func newTxForComparison(txId string, contractId *commonPb.Contract, parameterMap map[string]string) *commonPb.Transaction {
	var parameters []*commonPb.KeyValuePair
	for key, value := range parameterMap {
		parameters = append(parameters, &commonPb.KeyValuePair{
			Key:   key,
			Value: []byte(value),
		})
	}

	return &commonPb.Transaction{
		Payload: &commonPb.Payload{
			ChainId:        "chain1",
			TxType:         0,
			TxId:           txId,
			ContractName:   contractId.Name,
			Method:         "invoke",
			Parameters:     parameters,
			Timestamp:      time.Now().Unix(),
			ExpirationTime: 0,
			Limit:          &commonPb.Limit{GasLimit: 0},
		},
		Result: &commonPb.Result{
			Code: commonPb.TxStatusCode_SUCCESS,
			ContractResult: &commonPb.ContractResult{
				Code:    0,
				Result:  nil,
				Message: "",
			},
		},
		Sender: &commonPb.EndorsementEntry{
			Signer: &acPb.Member{
				OrgId:      "org1",
				MemberInfo: []byte("test-member"),
				MemberType: acPb.MemberType_CERT,
			},
			Signature: []byte("test-signature"),
		},
	}
}

// printComparisonRow 打印单行对比结果
func printComparisonRow(scenarioName string, wria, nd ComparisonMetrics) {
	ratio := wria.Throughput / nd.Throughput
	improvement := (ratio - 1) * 100

	fmt.Printf("  WRIA调度器:     吞吐量=%8.2f tx/s, 耗时=%8.2fms\n",
		wria.Throughput, float64(wria.TotalTime.Microseconds())/1000)
	fmt.Printf("  非确定性调度器: 吞吐量=%8.2f tx/s, 耗时=%8.2fms\n",
		nd.Throughput, float64(nd.TotalTime.Microseconds())/1000)

	if ratio > 1 {
		fmt.Printf("  ✓ WRIA 性能优于非确定性: %.2fx (提升 %.1f%%)\n", ratio, improvement)
	} else if ratio < 1 {
		fmt.Printf("  ✗ WRIA 性能低于非确定性: %.2fx (下降 %.1f%%)\n", ratio, -improvement)
	} else {
		fmt.Printf("  = 性能相当\n")
	}
}

// printComparisonSummary 打印汇总对比表
func printComparisonSummary(scenarioNames []string, wriaResults, ndResults []ComparisonMetrics) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("  性能对比汇总表")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n%-18s | %-15s | %-15s | %-12s\n", "场景", "WRIA(tx/s)", "非确定性(tx/s)", "性能比")
	fmt.Println(strings.Repeat("-", 80))

	for i := range scenarioNames {
		ratio := wriaResults[i].Throughput / ndResults[i].Throughput
		fmt.Printf("%-18s | %13.2f | %15.2f | %11.2fx\n",
			scenarioNames[i],
			wriaResults[i].Throughput,
			ndResults[i].Throughput,
			ratio)
	}

	// 计算平均性能比
	avgRatio := 0.0
	for i := range wriaResults {
		avgRatio += wriaResults[i].Throughput / ndResults[i].Throughput
	}
	avgRatio /= float64(len(wriaResults))

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-18s | %13s | %15s | %11.2fx\n", "平均性能比", "", "", avgRatio)

	fmt.Println("\n结论：")
	if avgRatio > 1.1 {
		fmt.Printf("  ✓ WRIA 调度器在多数场景下表现优于非确定性调度器 (平均 %.2fx)\n", avgRatio)
	} else if avgRatio < 0.9 {
		fmt.Printf("  ✗ WRIA 调度器在多数场景下表现低于非确定性调度器 (平均 %.2fx)\n", avgRatio)
	} else {
		fmt.Printf("  = 两个调度器性能相当 (平均 %.2fx)\n", avgRatio)
	}
}

// analyzeScalability 分析可扩展性
func analyzeScalability(schedulerName string, txCounts []int, results []ComparisonMetrics) {
	fmt.Printf("%s:\n", schedulerName)
	for i := 1; i < len(results); i++ {
		scaleFactor := float64(txCounts[i]) / float64(txCounts[i-1])
		throughputRatio := results[i].Throughput / results[i-1].Throughput
		efficiency := (throughputRatio / scaleFactor) * 100

		fmt.Printf("  %5d -> %5d: 交易量 %.2fx, 吞吐量 %.2fx (效率: %.1f%%)\n",
			txCounts[i-1], txCounts[i], scaleFactor, throughputRatio, efficiency)
	}
}
