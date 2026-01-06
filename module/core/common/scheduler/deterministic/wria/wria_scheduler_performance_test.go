/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package wria

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"chainmaker.org/chainmaker/logger/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"

	crypto2 "chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/pb-go/v2/consensus"
)

var (
	TestPrivKeyFile = "../../../../../config/wx-org1/certs/node/consensus1/consensus1.sign.key"
	TestCertFile    = "../../../../../config/wx-org1/certs/node/consensus1/consensus1.sign.crt"
)

// PerformanceMetrics 性能指标统计
type PerformanceMetrics struct {
	TotalTxs   int           // 总交易数
	TotalTime  time.Duration // 总耗时
	Throughput float64       // 吞吐量 (txs/s)
}

// TestWriaScheduler_Performance_NoConflict 测试无冲突场景的性能
// 场景：所有交易完全独立，无读写依赖
// 预期：高吞吐量，接近线性扩展
func TestWriaScheduler_Performance_NoConflict(t *testing.T) {
	fmt.Println("\n========== 测试场景：无冲突 ==========")

	testSizes := []int{1000}

	for _, txCount := range testSizes {
		t.Run(fmt.Sprintf("TxCount=%d", txCount), func(t *testing.T) {
			metrics := runPerformanceTest(t, txCount, ConflictScenarioNone)
			printMetrics(txCount, "无冲突", metrics)

			// 验证吞吐量合理性
			require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
		})
	}
}

// TestWriaScheduler_Performance_LowConflict 测试低冲突场景的性能
// 场景：10% 的交易有读写冲突（简化场景，所有冲突交易访问同一个共享key，且不打乱交易）
// 预期：中等吞吐量，少量 abort，rechecking 不能挽救交易
func TestWriaScheduler_Performance_LowConflict(t *testing.T) {
	fmt.Println("\n========== 测试场景：低冲突率 (10%) - 简化场景 ==========")

	testSizes := []int{1000}

	for _, txCount := range testSizes {
		t.Run(fmt.Sprintf("TxCount=%d", txCount), func(t *testing.T) {
			metrics := runPerformanceTest(t, txCount, ConflictScenarioLow)
			printMetrics(txCount, "低冲突(简化)", metrics)

			// 验证性能合理性
			require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
		})

	}
}

// TestWriaScheduler_Performance_LowConflictActual 测试真实低冲突场景的性能
// 场景：10% 的交易有读写冲突（真实场景，冲突分散在多个共享key上，且打乱交易）
// 预期：吞吐量应该比简化场景更高，因为冲突更分散
func TestWriaScheduler_Performance_LowConflictActual(t *testing.T) {
	fmt.Println("\n========== 测试场景：低冲突率 (10%) - 真实场景 ==========")

	testSizes := []int{1000}

	for _, txCount := range testSizes {
		t.Run(fmt.Sprintf("TxCount=%d", txCount), func(t *testing.T) {
			metrics := runPerformanceTest(t, txCount, ConflictScenarioLowActual)
			printMetrics(txCount, "低冲突(真实)", metrics)

			// 验证性能合理性
			require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
		})
	}
}

// TestWriaScheduler_Performance_HighConflict 测试高冲突场景的性能
// 场景：50% 的交易有读写冲突
// 预期：较低吞吐量，较多 abort，rechecking 机制无法发挥作用
func TestWriaScheduler_Performance_HighConflict(t *testing.T) {
	fmt.Println("\n========== 测试场景：高冲突率 (50%) ==========")

	testSizes := []int{1000}

	for _, txCount := range testSizes {
		t.Run(fmt.Sprintf("TxCount=%d", txCount), func(t *testing.T) {
			metrics := runPerformanceTest(t, txCount, ConflictScenarioHigh)
			printMetrics(txCount, "高冲突", metrics)

			// 验证性能合理性
			require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
		})
	}
}

// TestWriaScheduler_Performance_ChainDependency 测试链式依赖场景的性能
// 场景：交易形成依赖链 Tx0 -> Tx1 -> Tx2 -> ... -> TxN
// 预期：多轮调度，体现批处理优势
func TestWriaScheduler_Performance_ChainDependency(t *testing.T) {
	fmt.Println("\n========== 测试场景：链式依赖 ==========")

	testSizes := []int{1000}

	for _, txCount := range testSizes {
		t.Run(fmt.Sprintf("TxCount=%d", txCount), func(t *testing.T) {
			metrics := runPerformanceTest(t, txCount, ConflictScenarioChain)
			printMetrics(txCount, "链式依赖", metrics)

			// 验证性能合理性
			require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
		})
	}
}

// TestWriaScheduler_Performance_Scalability 测试可扩展性
// 测试不同交易量下的性能扩展性
func TestWriaScheduler_Performance_Scalability(t *testing.T) {
	fmt.Println("\n========== 可扩展性测试 ==========")

	testSizes := []int{50, 100, 200, 500, 1000, 2000, 5000}
	results := make([]PerformanceMetrics, len(testSizes))

	for i, txCount := range testSizes {
		metrics := runPerformanceTest(t, txCount, ConflictScenarioNone)
		results[i] = metrics
		fmt.Printf("TxCount=%5d | Throughput=%8.2f txs/s | TotalTime=%8.2fms\n",
			txCount, metrics.Throughput, float64(metrics.TotalTime.Microseconds())/1000)
	}

	// 分析可扩展性
	fmt.Println("\n可扩展性分析：")
	for i := 1; i < len(results); i++ {
		scaleFactor := float64(testSizes[i]) / float64(testSizes[i-1])
		throughputRatio := results[i].Throughput / results[i-1].Throughput
		fmt.Printf("  %5d -> %5d: 交易量增长 %.2fx, 吞吐量增长 %.2fx (效率: %.1f%%)\n",
			testSizes[i-1], testSizes[i], scaleFactor, throughputRatio, (throughputRatio/scaleFactor)*100)
	}
}

// TestWriaScheduler_Performance_ParallelismDegree 测试并行度
// 测试调度器对多核的利用效率
func TestWriaScheduler_Performance_ParallelismDegree(t *testing.T) {
	fmt.Println("\n========== 并行度测试 ==========")
	fmt.Printf("可用 CPU 核数: %d\n", runtime.NumCPU())

	txCount := 1000
	metrics := runPerformanceTest(t, txCount, ConflictScenarioNone)

	fmt.Printf("\n性能指标：\n")
	fmt.Printf("  总交易数: %d\n", metrics.TotalTxs)
	fmt.Printf("  总耗时: %v\n", metrics.TotalTime)
	fmt.Printf("  吞吐量: %.2f txs/s\n", metrics.Throughput)

	// 验证性能合理性
	require.Greater(t, metrics.Throughput, 0.0, "吞吐量应该大于0")
}

// TestWriaScheduler_Performance_BatchSizeImpact 测试批处理大小的影响
func TestWriaScheduler_Performance_BatchSizeImpact(t *testing.T) {
	fmt.Println("\n========== BatchSize 影响测试 ==========")
	fmt.Printf("当前 BatchSize: %d\n", BatchSize)

	txCount := 1000

	// 测试不同场景下的批处理表现
	scenarios := []ConflictScenario{
		ConflictScenarioNone,
		ConflictScenarioLow,
		ConflictScenarioHigh,
	}
	scenarioNames := []string{"无冲突", "低冲突", "高冲突"}

	fmt.Printf("\nBatchSize=%d 在不同场景下的表现：\n", BatchSize)
	fmt.Printf("%-12s | 吞吐量 (txs/s) | 总耗时 (ms)\n", "场景")
	fmt.Println("--------------------------------------------------")

	for i, scenario := range scenarios {
		metrics := runPerformanceTest(t, txCount, scenario)
		fmt.Printf("%-12s | %14.2f | %11.2f\n",
			scenarioNames[i],
			metrics.Throughput,
			float64(metrics.TotalTime.Microseconds())/1000)
	}
}

// ========== 辅助函数 ==========

// ConflictScenario 定义冲突场景类型
type ConflictScenario int

const (
	ConflictScenarioNone      ConflictScenario = iota // 无冲突
	ConflictScenarioLow                               // 低冲突 (10%) - 简化场景：所有冲突交易访问同一个共享key
	ConflictScenarioLowActual                         // 低冲突 (10%) - 真实场景：冲突分散在多个共享key上
	ConflictScenarioHigh                              // 高冲突 (50%)
	ConflictScenarioChain                             // 链式依赖
)

// runPerformanceTest 运行性能测试并返回指标
func runPerformanceTest(t *testing.T, txCount int, scenario ConflictScenario) PerformanceMetrics {
	// 设置日志级别为 INFO（只显示 INFO 及以上级别的日志，不显示 DEBUG）
	logConfig := logger.DefaultLogConfig()
	logConfig.SystemLog.LogLevelDefault = "DEBUG"
	logger.SetLogConfig(logConfig)

	// 配置
	localconf.ChainMakerConfig.NodeConfig.PrivKeyFile = TestPrivKeyFile
	localconf.ChainMakerConfig.NodeConfig.CertFile = TestCertFile
	localconf.ChainMakerConfig.NodeConfig.PrivKeyPassword = "11111111"

	// 准备环境
	snapshot, scheduler, block := prepareTestEnvironment(t, txCount, scenario)

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
	metrics := PerformanceMetrics{
		TotalTxs:   txCount,
		TotalTime:  totalTime,
		Throughput: float64(txCount) / totalTime.Seconds(),
	}

	return metrics
}

// prepareTestEnvironment 准备测试环境
func prepareTestEnvironment(t *testing.T, txCount int, scenario ConflictScenario) (
	protocol.Snapshot, protocol.TxScheduler, *commonPb.Block) {

	ctl := gomock.NewController(t)

	// 创建 mocks
	snapshot := mock.NewMockSnapshot(ctl)
	vmMgr := mock.NewMockVmManager(ctl)
	chainConf := mock.NewMockChainConf(ctl)
	storeHelper := mock.NewMockStoreHelper(ctl)
	ac := mock.NewMockAccessControlProvider(ctl)

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
		AuthType: protocol.Identity,
	}
	chainConf.EXPECT().ChainConfig().Return(chainConfig).AnyTimes()

	// 配置 StoreHelper
	storeHelper.EXPECT().GetPoolCapacity().Return(runtime.NumCPU() * 4).AnyTimes()

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
	setupSnapshotMocks(snapshot, txCount, chainConfig, ctl)

	// 配置 AccessControl
	ac.EXPECT().CreatePrincipal(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// 创建调度器
	scheduler := NewWriaScheduler(vmMgr, chainConf, storeHelper, ac)

	// 创建区块
	block := &commonPb.Block{
		Header: &commonPb.BlockHeader{
			ChainId:     "chain1",
			BlockHeight: 100,
		},
		Dag: &commonPb.DAG{},
	}

	return snapshot, scheduler, block
}

// setupSnapshotMocks 配置 Snapshot 的 mock 行为
func setupSnapshotMocks(snapshot *mock.MockSnapshot, txCount int, chainConfig *configpb.ChainConfig, ctl *gomock.Controller) {
	// 使用线程安全的 map 存储写入的数据
	var dataLock sync.RWMutex
	dataStore := make(map[string][]byte)

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

	snapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	snapshot.EXPECT().Seal().Return().AnyTimes()
	snapshot.EXPECT().GetTxTable().Return([]*commonPb.Transaction{}).AnyTimes()
	snapshot.EXPECT().GetTxRWSetTable().Return([]*commonPb.TxRWSet{}).AnyTimes()
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
		txs[i] = newTx(txId, contractId, make(map[string]string))
	}

	// 对于真实场景，打乱交易顺序以模拟真实的随机到达
	if scenario == ConflictScenarioLowActual {
		shuffleTransactions(txs)
	}

	return txs
}

// shuffleTransactions 使用 Fisher-Yates 算法打乱交易顺序
func shuffleTransactions(txs []*commonPb.Transaction) {
	// 使用当前时间作为随机种子，实现完全随机
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	n := len(txs)
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		txs[i], txs[j] = txs[j], txs[i]
	}
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
		// 低冲突（简化场景）：10% 的交易访问同一个共享 key
		if txIndex%10 == 0 {
			// 读写共享 key
			txSimContext.Get(contractName, []byte("shared_key"))
			txSimContext.Put(contractName, []byte("shared_key"), []byte(fmt.Sprintf("value_%d", txIndex)))
		} else {
			// 独立 key
			key := fmt.Sprintf("key_%d", txIndex)
			txSimContext.Put(contractName, []byte(key), []byte(fmt.Sprintf("value_%d", txIndex)))
		}

	case ConflictScenarioLowActual:
		// 低冲突（真实场景）：10% 的交易访问共享 key，但冲突分散在多个共享 key 上
		// 设置 10 个不同的共享 key，每个共享 key 会被约 1% 的交易访问
		const numSharedKeys = 10
		if txIndex%10 == 0 {
			// 将冲突交易分散到不同的共享 key 上
			// 使用 (txIndex / 10) % numSharedKeys 确保冲突分散
			sharedKeyIndex := (txIndex / 10) % numSharedKeys
			sharedKey := fmt.Sprintf("shared_key_%d", sharedKeyIndex)

			// 读写共享 key
			txSimContext.Get(contractName, []byte(sharedKey))
			txSimContext.Put(contractName, []byte(sharedKey), []byte(fmt.Sprintf("value_%d", txIndex)))
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

// newTx 创建测试交易
func newTx(txId string, contractId *commonPb.Contract, parameterMap map[string]string) *commonPb.Transaction {
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

// printMetrics 打印性能指标
func printMetrics(txCount int, scenario string, metrics PerformanceMetrics) {
	fmt.Printf("\n性能指标 [%s场景, %d笔交易]:\n", scenario, txCount)
	fmt.Printf("  总交易数: %d\n", metrics.TotalTxs)
	fmt.Printf("  总耗时: %v\n", metrics.TotalTime)
	fmt.Printf("  吞吐量: %.2f txs/s\n", metrics.Throughput)
	fmt.Println("----------------------------------------")
}
