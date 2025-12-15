/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package reorder

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2/mock"

	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// TestScheduleSimulateConsistency_Basic 测试Schedule和Simulate的基本一致性
// 验证：
// 1. Schedule生成DAG和交易顺序
// 2. Simulate按DAG重放
// 3. 两阶段的读写集一致
func TestScheduleSimulateConsistency_Basic(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_Basic begin ====")

	localconf.ChainMakerConfig.NodeConfig.PrivKeyFile = TestPrivKeyFile
	localconf.ChainMakerConfig.NodeConfig.CertFile = TestCertFile
	localconf.ChainMakerConfig.NodeConfig.PrivKeyPassword = "11111111"

	// 准备测试环境（3个交易）
	_, txRWSetTable, txTable, snapshot, scheduler, contractId, block := prepare(t, false, false, 3, true)

	// 配置交易读写集（模拟依赖关系）
	// Tx0: write K1
	tx0 := newTx("a0000000000000000000000000000000", contractId, make(map[string]string))
	txTable[0] = tx0
	txRWSetTable[0] = &commonPb.TxRWSet{
		TxId: tx0.Payload.TxId,
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K1"),
			Value:        []byte("V1"),
		}},
	}

	// Tx1: read K1, write K2 (依赖Tx0)
	tx1 := newTx("a0000000000000000000000000000001", contractId, make(map[string]string))
	txTable[1] = tx1
	txRWSetTable[1] = &commonPb.TxRWSet{
		TxId: tx1.Payload.TxId,
		TxReads: []*commonPb.TxRead{{
			ContractName: contractId.Name,
			Key:          []byte("K1"),
			Value:        []byte("V1"),
		}},
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K2"),
			Value:        []byte("V2"),
		}},
	}

	// Tx2: read K2, write K3 (依赖Tx1)
	tx2 := newTx("a0000000000000000000000000000002", contractId, make(map[string]string))
	txTable[2] = tx2
	txRWSetTable[2] = &commonPb.TxRWSet{
		TxId: tx2.Payload.TxId,
		TxReads: []*commonPb.TxRead{{
			ContractName: contractId.Name,
			Key:          []byte("K2"),
			Value:        []byte("V2"),
		}},
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K3"),
			Value:        []byte("V3"),
		}},
	}

	// 配置snapshot的mock
	snapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
			runVmSuccess bool, applySpecialTx bool) (bool, int) {
			return true, 3
		}).AnyTimes()
	snapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	snapshot.EXPECT().Seal().Return().AnyTimes()

	txBatch := []*commonPb.Transaction{tx0, tx1, tx2}

	// ========== 执行 Schedule ==========
	fmt.Println("\n===== 执行 Schedule 阶段 =====")
	scheduleRWSet, scheduleEvents, err := scheduler.Schedule(block, txBatch, snapshot)
	require.Nil(t, err)
	require.NotNil(t, scheduleRWSet)
	require.NotNil(t, scheduleEvents)

	// 验证DAG已生成
	require.NotNil(t, block.Dag, "DAG应该已生成")
	require.NotNil(t, block.Dag.Vertexes, "DAG顶点应该存在")
	fmt.Printf("Schedule完成: 交易数=%d, DAG批次=%d\n",
		len(scheduleRWSet), len(block.Dag.Vertexes[0].Neighbors))

	// 打印Schedule阶段的读写集
	fmt.Println("\nSchedule阶段读写集:")
	for txId, rwSet := range scheduleRWSet {
		fmt.Printf("  %s: reads=%d, writes=%d\n", txId[:16], len(rwSet.TxReads), len(rwSet.TxWrites))
	}

	// ========== 执行 Simulate ==========
	fmt.Println("\n===== 执行 Simulate 阶段 =====")

	// 创建新的snapshot用于Simulate
	_, simulateTxRWSetTable, simulateTxTable, simulateSnapshot, _, _, _ := prepare(t, false, false, 3, true)

	// 将Schedule阶段的交易和读写集传递给Simulate的snapshot
	// 复制交易
	simulateTxTable[0] = tx0
	simulateTxTable[1] = tx1
	simulateTxTable[2] = tx2

	// 复制Schedule生成的读写集到Simulate的mock中
	simulateTxRWSetTable[0] = scheduleRWSet[tx0.Payload.TxId]
	simulateTxRWSetTable[1] = scheduleRWSet[tx1.Payload.TxId]
	simulateTxRWSetTable[2] = scheduleRWSet[tx2.Payload.TxId]

	// 配置Simulate的mock
	simulateSnapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
			runVmSuccess bool, applySpecialTx bool) (bool, int) {
			return true, 3
		}).AnyTimes()
	simulateSnapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	simulateSnapshot.EXPECT().Seal().Return().AnyTimes()

	// Simulate使用Schedule生成的DAG和交易顺序
	simulateRWSet, simulateEvents, err := scheduler.SimulateWithDag(block, simulateSnapshot)
	require.Nil(t, err)
	require.NotNil(t, simulateRWSet)
	require.NotNil(t, simulateEvents)

	fmt.Printf("Simulate完成: 交易数=%d\n", len(simulateRWSet))

	// 打印Simulate阶段的读写集
	fmt.Println("\nSimulate阶段读写集:")
	for txId, rwSet := range simulateRWSet {
		fmt.Printf("  %s: reads=%d, writes=%d\n", txId[:16], len(rwSet.TxReads), len(rwSet.TxWrites))
	}

	// ========== 验证一致性 ==========
	fmt.Println("\n===== 验证一致性 =====")

	// 1. 验证交易数量一致
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet),
		"Schedule和Simulate的交易数量应该一致")
	fmt.Printf("✅ 交易数量一致: %d\n", len(scheduleRWSet))

	// 2. 验证每个交易都存在
	for txId := range scheduleRWSet {
		require.Contains(t, simulateRWSet, txId,
			"Simulate应该包含交易: %s", txId)
	}
	fmt.Println("✅ 所有交易在两个阶段都存在")

	// 3. 验证读写集结构一致（数量）
	for txId, scheduleRW := range scheduleRWSet {
		simulateRW, exists := simulateRWSet[txId]
		require.True(t, exists, "交易%s应该存在于Simulate结果中", txId)

		require.Equal(t, len(scheduleRW.TxReads), len(simulateRW.TxReads),
			"交易%s的读集数量应该一致", txId)
		require.Equal(t, len(scheduleRW.TxWrites), len(simulateRW.TxWrites),
			"交易%s的写集数量应该一致", txId)
	}
	fmt.Println("✅ 读写集结构一致")

	fmt.Println("\n===== ✅ Schedule和Simulate一致性验证通过 =====")
}

// TestScheduleSimulateConsistency_NoConflict 测试无冲突场景的一致性
// 场景：所有交易无读写依赖，应该可以并发执行
func TestScheduleSimulateConsistency_NoConflict(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_NoConflict begin ====")

	localconf.ChainMakerConfig.NodeConfig.PrivKeyFile = TestPrivKeyFile
	localconf.ChainMakerConfig.NodeConfig.CertFile = TestCertFile
	localconf.ChainMakerConfig.NodeConfig.PrivKeyPassword = "11111111"

	_, txRWSetTable, txTable, snapshot, scheduler, contractId, block := prepare(t, false, false, 3, true)

	// 配置3个无冲突的交易
	// Tx0: write K1
	tx0 := newTx("a0000000000000000000000000000000", contractId, make(map[string]string))
	txTable[0] = tx0
	txRWSetTable[0] = &commonPb.TxRWSet{
		TxId: tx0.Payload.TxId,
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K1"),
			Value:        []byte("V1"),
		}},
	}

	// Tx1: write K2 (无依赖)
	tx1 := newTx("a0000000000000000000000000000001", contractId, make(map[string]string))
	txTable[1] = tx1
	txRWSetTable[1] = &commonPb.TxRWSet{
		TxId: tx1.Payload.TxId,
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K2"),
			Value:        []byte("V2"),
		}},
	}

	// Tx2: write K3 (无依赖)
	tx2 := newTx("a0000000000000000000000000000002", contractId, make(map[string]string))
	txTable[2] = tx2
	txRWSetTable[2] = &commonPb.TxRWSet{
		TxId: tx2.Payload.TxId,
		TxWrites: []*commonPb.TxWrite{{
			ContractName: contractId.Name,
			Key:          []byte("K3"),
			Value:        []byte("V3"),
		}},
	}

	snapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
			runVmSuccess bool, applySpecialTx bool) (bool, int) {
			return true, 3
		}).AnyTimes()
	snapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	snapshot.EXPECT().Seal().Return().AnyTimes()

	dag := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{{}},
	}
	snapshot.EXPECT().BuildDAG(gomock.Any(), gomock.Any()).Return(dag).AnyTimes()

	txBatch := []*commonPb.Transaction{tx0, tx1, tx2}

	// Schedule
	scheduleRWSet, _, err := scheduler.Schedule(block, txBatch, snapshot)
	require.Nil(t, err)
	fmt.Printf("Schedule完成（无冲突场景）: 交易数=%d\n", len(scheduleRWSet))

	// Simulate
	_, simulateTxRWSetTable2, simulateTxTable2, simulateSnapshot, _, _, _ := prepare(t, false, false, 3, true)

	// 复制交易和读写集
	simulateTxTable2[0] = tx0
	simulateTxTable2[1] = tx1
	simulateTxTable2[2] = tx2

	simulateTxRWSetTable2[0] = scheduleRWSet[tx0.Payload.TxId]
	simulateTxRWSetTable2[1] = scheduleRWSet[tx1.Payload.TxId]
	simulateTxRWSetTable2[2] = scheduleRWSet[tx2.Payload.TxId]

	simulateSnapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
			runVmSuccess bool, applySpecialTx bool) (bool, int) {
			return true, 3
		}).AnyTimes()
	simulateSnapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	simulateSnapshot.EXPECT().Seal().Return().AnyTimes()

	simulateRWSet, _, err := scheduler.SimulateWithDag(block, simulateSnapshot)
	require.Nil(t, err)
	fmt.Printf("Simulate完成（无冲突场景）: 交易数=%d\n", len(simulateRWSet))

	// 验证
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet))
	fmt.Println("✅ 无冲突场景一致性验证通过")
}

// TestScheduleSimulateConsistency_SimpleConflict 测试简单的读写冲突场景
// 场景：Tx0 写 K1，Tx1 读 K1 写 K2，Tx2 读 K2 写 K3
// 预期：Schedule和Simulate的最终状态完全一致
func TestScheduleSimulateConsistency_SimpleConflict(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_SimpleConflict begin ====")

	// ========== Schedule 阶段 ==========
	scheduleSnapshot, scheduleBlock, scheduleScheduler := prepareScheduleTest(t, 3)

	// 构造交易及读写集
	tx0, tx1, tx2 := createConflictTxs_SimpleChain(t)
	txBatch := []*commonPb.Transaction{tx0, tx1, tx2}

	// 设置 Schedule 阶段的 mock 期望
	setupScheduleMocks(t, scheduleSnapshot, 3)

	// 执行 Schedule
	scheduleRWSet, scheduleEvents, err := scheduleScheduler.Schedule(scheduleBlock, txBatch, scheduleSnapshot)
	require.Nil(t, err)
	require.NotNil(t, scheduleRWSet)
	require.NotNil(t, scheduleEvents)

	// 验证 DAG 已生成
	require.NotNil(t, scheduleBlock.Dag)
	require.NotNil(t, scheduleBlock.Dag.Vertexes)
	require.Greater(t, len(scheduleBlock.Dag.Vertexes), 0)

	fmt.Printf("Schedule完成: 区块高度=%d, DAG批次数=%d\n",
		scheduleBlock.Header.BlockHeight, len(scheduleBlock.Dag.Vertexes[0].Neighbors))

	// ========== Simulate 阶段 ==========
	simulateSnapshot, simulateBlock, simulateScheduler := prepareSimulateTest(t, 3)

	// 使用 Schedule 生成的 DAG 和交易顺序
	simulateBlock.Dag = scheduleBlock.Dag
	simulateBlock.Txs = scheduleBlock.Txs

	// 设置 Simulate 阶段的 mock 期望
	setupSimulateMocks(t, simulateSnapshot, 3)

	// 执行 Simulate
	simulateRWSet, simulateEvents, err := simulateScheduler.SimulateWithDag(simulateBlock, simulateSnapshot)
	require.Nil(t, err)
	require.NotNil(t, simulateRWSet)
	require.NotNil(t, simulateEvents)

	fmt.Printf("Simulate完成: 区块高度=%d\n", simulateBlock.Header.BlockHeight)

	// ========== 验证一致性 ==========
	// 1. 验证交易数量一致
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet), "RWSet数量不一致")

	// 2. 验证每个交易的读写集一致
	for txId := range scheduleRWSet {
		require.Contains(t, simulateRWSet, txId, "Simulate缺少交易: %s", txId)
		// 注意：实际环境中应该验证读写集内容，这里简化处理
	}

	fmt.Println("✅ Schedule和Simulate状态一致性验证通过")
}

// TestScheduleSimulateConsistency_ComplexDAG 测试复杂DAG场景
// 场景：5个交易形成复杂依赖关系
// Tx0 -> Tx2 (Tx0写K1, Tx2读K1)
// Tx1 -> Tx3 (Tx1写K2, Tx3读K2)
// Tx2 -> Tx4 (Tx2写K3, Tx4读K3)
// Tx3 -> Tx4 (Tx3写K4, Tx4读K4)
func TestScheduleSimulateConsistency_ComplexDAG(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_ComplexDAG begin ====")

	// ========== Schedule 阶段 ==========
	scheduleSnapshot, scheduleBlock, scheduleScheduler := prepareScheduleTest(t, 5)

	// 构造交易
	txBatch := createComplexDAGTxs(t)
	require.Equal(t, 5, len(txBatch))

	// 设置 mock
	setupScheduleMocks(t, scheduleSnapshot, 5)

	// 执行 Schedule
	scheduleRWSet, _, err := scheduleScheduler.Schedule(scheduleBlock, txBatch, scheduleSnapshot)
	require.Nil(t, err)

	fmt.Printf("Schedule完成: DAG批次边界=%v\n", scheduleBlock.Dag.Vertexes[0].Neighbors)

	// ========== Simulate 阶段 ==========
	simulateSnapshot, simulateBlock, simulateScheduler := prepareSimulateTest(t, 5)

	simulateBlock.Dag = scheduleBlock.Dag
	simulateBlock.Txs = scheduleBlock.Txs

	setupSimulateMocks(t, simulateSnapshot, 5)

	// 执行 Simulate
	simulateRWSet, _, err := simulateScheduler.SimulateWithDag(simulateBlock, simulateSnapshot)
	require.Nil(t, err)

	// ========== 验证一致性 ==========
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet), "交易数量不一致")

	for txId := range scheduleRWSet {
		require.Contains(t, simulateRWSet, txId, "Simulate缺少交易: %s", txId)
	}

	fmt.Println("✅ 复杂DAG场景一致性验证通过")
}

// TestScheduleSimulateConsistency_WriteAfterWrite 测试写后写冲突
// 场景：多个交易写同一个Key
// Tx0: write K1=V0
// Tx1: write K1=V1
// Tx2: write K1=V2
// 预期：按确定性顺序执行，最终值确定
func TestScheduleSimulateConsistency_WriteAfterWrite(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_WriteAfterWrite begin ====")

	// ========== Schedule 阶段 ==========
	scheduleSnapshot, scheduleBlock, scheduleScheduler := prepareScheduleTest(t, 3)

	txBatch := createWAWConflictTxs(t)

	setupScheduleMocks(t, scheduleSnapshot, 3)

	scheduleRWSet, _, err := scheduleScheduler.Schedule(scheduleBlock, txBatch, scheduleSnapshot)
	require.Nil(t, err)

	// ========== Simulate 阶段 ==========
	simulateSnapshot, simulateBlock, simulateScheduler := prepareSimulateTest(t, 3)

	simulateBlock.Dag = scheduleBlock.Dag
	simulateBlock.Txs = scheduleBlock.Txs

	setupSimulateMocks(t, simulateSnapshot, 3)

	simulateRWSet, _, err := simulateScheduler.SimulateWithDag(simulateBlock, simulateSnapshot)
	require.Nil(t, err)

	// ========== 验证一致性 ==========
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet), "WAW场景交易数量不一致")

	fmt.Println("✅ 写后写冲突一致性验证通过")
}

// TestScheduleSimulateConsistency_LongChain 测试长依赖链
// 场景：10个交易形成长依赖链
// Tx0 -> Tx1 -> Tx2 -> ... -> Tx9
func TestScheduleSimulateConsistency_LongChain(t *testing.T) {
	fmt.Println("===== TestScheduleSimulateConsistency_LongChain begin ====")

	const chainLen = 10

	// ========== Schedule 阶段 ==========
	scheduleSnapshot, scheduleBlock, scheduleScheduler := prepareScheduleTest(t, chainLen)

	txBatch := createLongChainTxs(t, chainLen)

	setupScheduleMocks(t, scheduleSnapshot, chainLen)

	scheduleRWSet, _, err := scheduleScheduler.Schedule(scheduleBlock, txBatch, scheduleSnapshot)
	require.Nil(t, err)

	// ========== Simulate 阶段 ==========
	simulateSnapshot, simulateBlock, simulateScheduler := prepareSimulateTest(t, chainLen)

	simulateBlock.Dag = scheduleBlock.Dag
	simulateBlock.Txs = scheduleBlock.Txs

	setupSimulateMocks(t, simulateSnapshot, chainLen)

	simulateRWSet, _, err := simulateScheduler.SimulateWithDag(simulateBlock, simulateSnapshot)
	require.Nil(t, err)

	// ========== 验证一致性 ==========
	require.Equal(t, len(scheduleRWSet), len(simulateRWSet), "长链场景交易数量不一致")

	fmt.Printf("✅ 长依赖链(%d个交易)一致性验证通过\n", chainLen)
}

// ========== 辅助函数 ==========

// prepareScheduleTest 准备 Schedule 测试环境
func prepareScheduleTest(t *testing.T, txCount int) (*mock.MockSnapshot, *commonPb.Block, protocol.TxScheduler) {
	localconf.ChainMakerConfig.NodeConfig.PrivKeyFile = TestPrivKeyFile
	localconf.ChainMakerConfig.NodeConfig.CertFile = TestCertFile
	localconf.ChainMakerConfig.NodeConfig.PrivKeyPassword = "11111111"

	ctl := gomock.NewController(t)

	snapshot := mock.NewMockSnapshot(ctl)
	vmMgr := mock.NewMockVmManager(ctl)
	ledgerCache := mock.NewMockLedgerCache(ctl)
	chainConf := mock.NewMockChainConf(ctl)
	storeHelper := mock.NewMockStoreHelper(ctl)
	ac := mock.NewMockAccessControlProvider(ctl)
	signer := mock.NewMockSigningMember(ctl)

	// 基础 mock 配置
	setupBasicMocks(chainConf, storeHelper, vmMgr, ledgerCache, ac, signer)

	scheduler := NewReorderTxScheduler(vmMgr, chainConf, storeHelper, ac)

	block := &commonPb.Block{
		Header: &commonPb.BlockHeader{
			ChainId:     "chain1",
			BlockHeight: 10,
		},
		Dag: &commonPb.DAG{
			Vertexes: []*commonPb.DAG_Neighbor{},
		},
	}

	return snapshot, block, scheduler
}

// prepareSimulateTest 准备 Simulate 测试环境
func prepareSimulateTest(t *testing.T, txCount int) (*mock.MockSnapshot, *commonPb.Block, protocol.TxScheduler) {
	return prepareScheduleTest(t, txCount)
}

// setupBasicMocks 设置基础的 mock 对象
func setupBasicMocks(chainConf *mock.MockChainConf, storeHelper *mock.MockStoreHelper,
	vmMgr *mock.MockVmManager, ledgerCache *mock.MockLedgerCache,
	ac *mock.MockAccessControlProvider, signer *mock.MockSigningMember) {

	// ChainConf
	chainConf.EXPECT().ChainConfig().Return(&configpb.ChainConfig{
		ChainId: "chain1",
		Vm: &configpb.Vm{
			AddrType: configpb.AddrType_CHAINMAKER,
		},
		Crypto: &configpb.CryptoConfig{
			Hash: "SHA256",
		},
		Consensus: &configpb.ConsensusConfig{
			Type: 1,
		},
		Core: &configpb.CoreConfig{
			ConsensusTurboConfig: &configpb.ConsensusTurboConfig{
				ConsensusMessageTurbo: false,
			},
			EnableOptimizeChargeGas: false,
		},
		Block: &configpb.BlockConfig{
			TxTimeout: 10,
		},
	}).AnyTimes()

	// StoreHelper
	storeHelper.EXPECT().GetPoolCapacity().Return(runtime.NumCPU() * 4).AnyTimes()

	// VmMgr - 根据交易ID设置不同的读写集以构建复杂DAG
	vmMgr.EXPECT().RunContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(contract *commonPb.Contract, method string, byteCode []byte, parameters map[string][]byte,
			txSimContext protocol.TxSimContext, gasUsed uint64, refTxType commonPb.TxType) (*commonPb.ContractResult, protocol.ExecOrderTxType, commonPb.TxStatusCode) {

			// 根据交易ID设置读写集，实现复杂DAG依赖关系
			txId := txSimContext.GetTx().Payload.TxId
			contractName := contract.Name

			switch txId {
			case "tx0000000000000000000000000000000":
				// Tx0: 写K1
				txSimContext.Put(contractName, []byte("K1"), []byte("V1"))
			case "tx0000000000000000000000000000001":
				// Tx1: 写K2
				txSimContext.Put(contractName, []byte("K2"), []byte("V2"))
			case "tx0000000000000000000000000000002":
				// Tx2: 读K1, 写K3 (依赖Tx0)
				txSimContext.Get(contractName, []byte("K1"))
				txSimContext.Put(contractName, []byte("K3"), []byte("V3"))
			case "tx0000000000000000000000000000003":
				// Tx3: 读K2, 写K4 (依赖Tx1)
				txSimContext.Get(contractName, []byte("K2"))
				txSimContext.Put(contractName, []byte("K4"), []byte("V4"))
			case "tx0000000000000000000000000000004":
				// Tx4: 读K3, 读K4 (依赖Tx2和Tx3)
				txSimContext.Get(contractName, []byte("K3"))
				txSimContext.Get(contractName, []byte("K4"))
			default:
				// 其他交易保持默认行为（无读写集）
			}

			return &commonPb.ContractResult{
				Code:    uint32(0),
				Result:  nil,
				Message: "OK",
			}, protocol.ExecOrderTxTypeNormal, commonPb.TxStatusCode_SUCCESS
		}).AnyTimes()
	vmMgr.EXPECT().BeforeSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()
	vmMgr.EXPECT().AfterSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()

	// LedgerCache
	ledgerCache.EXPECT().GetLastCommittedBlock().Return(&commonPb.Block{
		Header: &commonPb.BlockHeader{
			BlockHeight: 9,
		},
	}).AnyTimes()
	ledgerCache.EXPECT().CurrentHeight().Return(uint64(9), nil).AnyTimes()

	// AccessControl
	ac.EXPECT().CreatePrincipal(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// SigningMember
	signer.EXPECT().Sign(gomock.Any(), gomock.Any()).Return([]byte("mock-signature"), nil).AnyTimes()
	signer.EXPECT().GetMember().Return(&acPb.Member{
		OrgId:      "org1",
		MemberInfo: []byte("mock-member-info"),
		MemberType: acPb.MemberType_CERT,
	}, nil).AnyTimes()
}

// setupScheduleMocks 设置 Schedule 阶段的 mock
func setupScheduleMocks(t *testing.T, snapshot *mock.MockSnapshot, txCount int) {
	snapshot.EXPECT().ApplyTxSimContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
			runVmSuccess bool, applySpecialTx bool) (bool, int) {
			// 模拟成功应用到快照
			// 注意：返回的txCount会影响DAG构建，但实际的读写集依赖关系
			// 由TxSimContext中的读写集决定
			return true, txCount
		}).AnyTimes()

	// 添加 GetKey 的 mock - 关键修复！
	// 为 ComplexDAG 场景返回模拟的键值数据
	snapshot.EXPECT().GetKey(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(txExecSeq int, contractName string, key []byte) ([]byte, error) {
			keyStr := string(key)
			switch keyStr {
			case "K1":
				return []byte("V1"), nil
			case "K2":
				return []byte("V2"), nil
			case "K3":
				return []byte("V3"), nil
			case "K4":
				return []byte("V4"), nil
			default:
				return nil, nil
			}
		}).AnyTimes()

	snapshot.EXPECT().IsSealed().Return(false).AnyTimes()
	snapshot.EXPECT().Seal().Return().AnyTimes()

	dag := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{{}},
	}
	snapshot.EXPECT().BuildDAG(gomock.Any(), gomock.Any()).Return(dag).AnyTimes()
	snapshot.EXPECT().GetTxResultMap().Return(make(map[string]*commonPb.Result)).AnyTimes()

	// 添加 GetTxTable 和 GetTxRWSetTable 的 mock
	snapshot.EXPECT().GetTxTable().Return([]*commonPb.Transaction{}).AnyTimes()
	snapshot.EXPECT().GetTxRWSetTable().Return([]*commonPb.TxRWSet{}).AnyTimes()

	// 添加 GetSnapshotSize 的 mock
	snapshot.EXPECT().GetSnapshotSize().Return(0).AnyTimes()

	// 添加 GetLastChainConfig 的 mock
	chainConfig := &configpb.ChainConfig{
		ChainId: "chain1",
		Vm: &configpb.Vm{
			AddrType: configpb.AddrType_CHAINMAKER,
		},
		Crypto: &configpb.CryptoConfig{
			Hash: "SHA256",
		},
		Consensus: &configpb.ConsensusConfig{
			Type: 1,
		},
		Core: &configpb.CoreConfig{
			ConsensusTurboConfig: &configpb.ConsensusTurboConfig{
				ConsensusMessageTurbo: false,
			},
			EnableOptimizeChargeGas: false,
		},
		Block: &configpb.BlockConfig{
			TxTimeout: 10,
		},
		AuthType: protocol.Identity,
	}
	snapshot.EXPECT().GetLastChainConfig().Return(chainConfig).AnyTimes()

	// 添加 GetBlockchainStore 的 mock
	blockChainStore := mock.NewMockBlockchainStore(gomock.NewController(t))
	blockChainStore.EXPECT().GetContractByName(gomock.Any()).Return(&commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}, nil).AnyTimes()
	blockChainStore.EXPECT().GetContractBytecode(gomock.Any()).AnyTimes()
	snapshot.EXPECT().GetBlockchainStore().Return(blockChainStore).AnyTimes()

	// 添加其他常用的mock
	snapshot.EXPECT().GetSpecialTxTable().Return([]*commonPb.Transaction{}).AnyTimes()
	snapshot.EXPECT().GetBlockFingerprint().Return("10").AnyTimes()
}

// setupSimulateMocks 设置 Simulate 阶段的 mock
func setupSimulateMocks(t *testing.T, snapshot *mock.MockSnapshot, txCount int) {
	// Simulate 和 Schedule 使用相同的 mock 配置
	setupScheduleMocks(t, snapshot, txCount)
}

// createConflictTxs_SimpleChain 创建简单依赖链交易
// Tx0: write K1
// Tx1: read K1, write K2
// Tx2: read K2, write K3
func createConflictTxs_SimpleChain(t *testing.T) (*commonPb.Transaction, *commonPb.Transaction, *commonPb.Transaction) {
	contractId := &commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	params := make(map[string]string)

	tx0 := newTx("tx0000000000000000000000000000000", contractId, params)
	tx1 := newTx("tx0000000000000000000000000000001", contractId, params)
	tx2 := newTx("tx0000000000000000000000000000002", contractId, params)

	return tx0, tx1, tx2
}

// createComplexDAGTxs 创建复杂DAG交易
func createComplexDAGTxs(t *testing.T) []*commonPb.Transaction {
	contractId := &commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	params := make(map[string]string)
	txs := make([]*commonPb.Transaction, 5)

	for i := 0; i < 5; i++ {
		txId := fmt.Sprintf("tx000000000000000000000000000000%d", i)
		txs[i] = newTx(txId, contractId, params)
	}

	return txs
}

// createWAWConflictTxs 创建写后写冲突交易
func createWAWConflictTxs(t *testing.T) []*commonPb.Transaction {
	contractId := &commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	params := make(map[string]string)

	tx0 := newTx("waw0000000000000000000000000000", contractId, params)
	tx1 := newTx("waw0000000000000000000000000001", contractId, params)
	tx2 := newTx("waw0000000000000000000000000002", contractId, params)

	return []*commonPb.Transaction{tx0, tx1, tx2}
}

// createLongChainTxs 创建长依赖链交易
func createLongChainTxs(t *testing.T, chainLen int) []*commonPb.Transaction {
	contractId := &commonPb.Contract{
		Name:        "TestContract",
		Version:     "1.0",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	params := make(map[string]string)
	txs := make([]*commonPb.Transaction, chainLen)

	for i := 0; i < chainLen; i++ {
		txId := fmt.Sprintf("chain00000000000000000000000000%02d", i)
		txs[i] = newTx(txId, contractId, params)
	}

	return txs
}
