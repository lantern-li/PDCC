/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifier

import (
	"encoding/hex"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	commonErrors "chainmaker.org/chainmaker/common/v2/errors"
	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2"
	batch "chainmaker.org/chainmaker/txpool-batch/v2"
	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker-go/module/consensus"
	"chainmaker.org/chainmaker-go/module/core/common"
	"chainmaker.org/chainmaker-go/module/core/common/coinbasemgr"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
)

// DeterministicBlockVerifierImpl implements BlockVerifier interface.
// Verify block and transactions.
// nolint: structcheck,unused
type DeterministicBlockVerifierImpl struct {
	chainId         string                   // chain id, to identity this chain
	msgBus          msgbus.MessageBus        // message bus
	txScheduler     protocol.TxScheduler     // scheduler orders tx batch into DAG form and returns a block
	snapshotManager protocol.SnapshotManager // snapshot manager
	ledgerCache     protocol.LedgerCache     // ledger cache
	blockchainStore protocol.BlockchainStore // blockchain store

	reentrantLocks *common.ReentrantLocks         // reentrant lock for avoid concurrent verify block
	proposalCache  protocol.ProposalCache         // proposal cache
	chainConf      protocol.ChainConf             // chain config
	ac             protocol.AccessControlProvider // access control manager
	log            protocol.Logger                // logger
	txPool         protocol.TxPool                // tx pool to check if tx is duplicate
	txFilter       protocol.TxFilter              // tx pool to check if tx is duplicate
	// mu             sync.Mutex                     // to avoid concurrent map modify
	verifierBlock *common.VerifierBlock
	storeHelper   conf.StoreHelper

	metricBlockVerifyTime *prometheus.HistogramVec // metrics monitor
	netService            protocol.NetService
}

func NewDeterministicBlockVerifier(config BlockVerifierConfig, log protocol.Logger, metricBlockVerifyTime *prometheus.HistogramVec) (BlockVerifier, error) {
	v := &DeterministicBlockVerifierImpl{
		chainId:         config.ChainId,
		msgBus:          config.MsgBus,
		txScheduler:     config.TxScheduler,
		snapshotManager: config.SnapshotManager,
		ledgerCache:     config.LedgerCache,
		blockchainStore: config.BlockchainStore,
		reentrantLocks: &common.ReentrantLocks{
			ReentrantLocks: make(map[string]interface{}),
		},
		proposalCache:         config.ProposedCache,
		chainConf:             config.ChainConf,
		ac:                    config.AC,
		log:                   log,
		txPool:                config.TxPool,
		storeHelper:           config.StoreHelper,
		netService:            config.NetService,
		txFilter:              config.TxFilter,
		metricBlockVerifyTime: metricBlockVerifyTime,
	}

	verifyConf := &common.VerifierBlockConf{
		ChainConf:       config.ChainConf,
		Log:             log,
		LedgerCache:     config.LedgerCache,
		Ac:              config.AC,
		SnapshotManager: config.SnapshotManager,
		VmMgr:           config.VmMgr,
		TxPool:          config.TxPool,
		BlockchainStore: config.BlockchainStore,
		ProposalCache:   config.ProposedCache,
		StoreHelper:     config.StoreHelper,
		TxScheduler:     config.TxScheduler,
		TxFilter:        config.TxFilter,
	}
	v.verifierBlock = common.NewVerifierBlock(verifyConf)

	config.MsgBus.Register(msgbus.ChainConfig, v) // todo 确认为什么需要这个topic

	return v, nil
}

// VerifyBlockSync only maxbft use this method
func (v *DeterministicBlockVerifierImpl) VerifyBlockSync(block *commonpb.Block,
	mode protocol.VerifyMode,
) (*consensuspb.VerifyResult, error) {
	return v.verifyBlockWithoutDag(block, mode)
}

// VerifyBlock to check if block is valid
func (v *DeterministicBlockVerifierImpl) VerifyBlock(block *commonpb.Block, mode protocol.VerifyMode) (err error) {
	/**
		ps： 确定性调度将verifyMode 分为了三种
		CONSENSUS_VERIFY：从节点共识验证； SYNC_VERIFY： 同步验证； PROPOSER_VERIFY： 提案节点共识验证。
		1. consensus 模式下，走schedule、finalize
		2. sync 模式下...... schedule、verify
	**/

	if v.chainConf.ChainConfig().Scheduler.ProcessType == configpb.ProcessType_EXECUTE_ON_PROPOSE && mode != protocol.SYNC_VERIFY {
		verifyResult, err := v.verifyBlockWithoutDag(block, mode)
		if err != nil {
			v.log.Error(err)
			return err
		}

		// 从节点通过msgbus将执行结果返回给consensus
		if mode == protocol.CONSENSUS_VERIFY {
			v.msgBus.Publish(msgbus.VerifyResult, verifyResult)
		}

		return nil
	}

	return v.verifyBlock(block, mode)
}

// VerifyBlockWithRwSets to check if block is valid
func (v *DeterministicBlockVerifierImpl) VerifyBlockWithRwSets(block *commonpb.Block,
	rwsets []*commonpb.TxRWSet, mode protocol.VerifyMode,
) (err error) {
	// 未开启快速校验或者主节点未将读写集带入，则走原Verify block 逻辑
	if len(rwsets) == 0 || mode != protocol.SYNC_VERIFY {
		return v.VerifyBlock(block, mode)
	}

	// 原快速同步逻辑，无需执行交易 && 检查读写集（通过merkle root、sign、vote保证正确性）
	return v.verifyBlockWithoutExecuting(block, rwsets, mode)
}

// VerifyBlockWithRwSets to check if block is valid
func (v *DeterministicBlockVerifierImpl) verifyBlockWithoutExecuting(block *commonpb.Block,
	rwsets []*commonpb.TxRWSet, mode protocol.VerifyMode,
) (err error) {
	startTick := utils.CurrentTimeMillisSeconds()
	if err = utils.IsEmptyBlock(block); err != nil {
		v.log.Error(err)
		v.log.Debugf("empty block. height:%+v, hash:%+v, chainId:%+v, preHash:%+v, signature:%+v",
			block.Header.BlockHeight, block.Header.BlockHash,
			block.Header.ChainId, block.Header.PreBlockHash, block.Header.Signature)
		return err
	}

	v.log.Debugf("verify receive [%d](%x,%d,%d), from sync %d",
		block.Header.BlockHeight, block.Header.BlockHash, block.Header.TxCount, len(block.Txs), mode)
	// avoid concurrent verify, only one block hash can be verified at the same time
	if !v.reentrantLocks.Lock(string(block.Header.BlockHash)) {
		v.log.Warnf("block(%d,%x) concurrent verify, yield", block.Header.BlockHeight, block.Header.BlockHash)
		return commonErrors.ErrConcurrentVerify
	}
	defer v.reentrantLocks.Unlock(string(block.Header.BlockHash))

	// No duplicate verify
	isRepeat := v.verifyRepeat(block, mode)
	if isRepeat {
		elapsed := utils.CurrentTimeMillisSeconds() - startTick
		v.log.Infof("verify success repeat [%d](%x), total: %d. mode:%d",
			block.Header.BlockHeight, block.Header.BlockHash, elapsed, mode)
		return nil
	}

	var contractEventMap map[string][]*commonpb.ContractEvent
	txRWSetMap := make(map[string]*commonpb.TxRWSet)
	for _, txRWSet := range rwsets {
		if txRWSet != nil {
			txRWSetMap[txRWSet.TxId] = txRWSet
		}
	}

	// avoid to recover the committed block.
	lastBlock, err := v.verifierBlock.FetchLastBlock(block)
	if err != nil {
		return err
	}

	// startPoolTick := utils.CurrentTimeMillisSeconds()
	newBlock, batchIds, err := common.RecoverBlock(block, mode, v.chainConf, v.txPool, v.ac, v.netService, v.log)
	if err != nil {
		return err
	}
	// lastPool := utils.CurrentTimeMillisSeconds() - startPoolTick
	contractEventMap, err = v.validateBlockWithRWSets(newBlock, lastBlock, mode, txRWSetMap)
	if err != nil {
		v.log.Warnf("verify failed [%d](%x),preBlockHash:%x, %s",
			newBlock.Header.BlockHeight, newBlock.Header.BlockHash, newBlock.Header.PreBlockHash, err.Error())

		// rollback sql
		if sqlErr := v.storeHelper.RollBack(newBlock, v.blockchainStore); sqlErr != nil {
			v.log.Errorf("block [%d] rollback sql failed: %s", newBlock.Header.BlockHeight, sqlErr)
		}
		return err
	}

	// sync mode, need to verify consensus vote signature
	// beginConsensCheck := utils.CurrentTimeMillisSeconds()
	if protocol.SYNC_VERIFY == mode {
		if err = v.verifyVoteSig(newBlock); err != nil {
			v.log.Warnf("verify failed [%d](%x), votesig %s",
				newBlock.Header.BlockHeight, newBlock.Header.BlockHash, err.Error())
			return err
		}
	}
	// consensusCheckUsed := utils.CurrentTimeMillisSeconds() - beginConsensCheck

	// verify success, cache block and read write set
	// solo need this，too！！！
	v.log.Debugf("set proposed block(%d,%x)", newBlock.Header.BlockHeight, newBlock.Header.BlockHash)
	if err = v.proposalCache.SetProposedBlock(&protocol.ProposalData{
		Block:            newBlock,
		TxRwSetMap:       txRWSetMap,
		ContractEventMap: contractEventMap,
	}, false); err != nil {
		return err
	}

	// mark transactions in block as pending status in txpool
	if common.TxPoolType == batch.TxPoolType {
		v.txPool.AddTxBatchesToPendingCache(batchIds, newBlock.Header.BlockHeight)
	} else {
		v.txPool.AddTxsToPendingCache(newBlock.Txs, newBlock.Header.BlockHeight)
	}

	//if protocol.CONSENSUS_VERIFY == mode {
	//	v.msgBus.Publish(msgbus.VerifyResult, parseVerifyResult(newBlock, true, txRWSetMap, nil))
	//}

	// elapsed := utils.CurrentTimeMillisSeconds() - startTick
	// v.log.Infof("verify success [%d,%x]"+
	//	"(blockSig:%d,vm:%d,txVerify:%d,txRoot:%d,pool:%d,consensusCheckUsed:%d,total:%d)",
	//	newBlock.Header.BlockHeight, newBlock.Header.BlockHash, timeLasts[common.Time_Statistics_BlockSig],
	//	timeLasts[common.Time_Statistics_VM], timeLasts[common.Time_Statistics_TxVerify],
	//	timeLasts[common.Time_Statistics_TxRoot], lastPool, consensusCheckUsed, elapsed)

	//if localconf.ChainMakerConfig.MonitorConfig.Enabled {
	//	v.metricBlockVerifyTime.WithLabelValues(v.chainId).Observe(float64(elapsed) / 1000)
	//}
	return nil
}

var _ msgbus.Subscriber = (*BlockVerifierImpl)(nil)

// OnMessage contract event data is a []string, hexToString(proto.Marshal(data))
func (v *DeterministicBlockVerifierImpl) OnMessage(msg *msgbus.Message) {
	//switch msg.Topic {
	//case msgbus.ChainConfig:
	//	dataStr, ok := msg.Payload.([]string)
	//	if !ok {
	//		return
	//	}
	//	dataBytes, err := hex.DecodeString(dataStr[0])
	//	if err != nil {
	//		v.log.Warn(err)
	//		return
	//	}
	//	chainConfig := &chainConfConfig.ChainConfig{}
	//	err = proto.Unmarshal(dataBytes, chainConfig)
	//	if err != nil {
	//		v.log.Warn(err)
	//		return
	//	}
	//	v.chainConf.ChainConfig().Block = chainConfig.Block
	//	protocol.ParametersValueMaxLength = chainConfig.Block.TxParameterSize * 1024 * 1024
	//	if chainConfig.Block.TxParameterSize <= 0 {
	//		protocol.ParametersValueMaxLength = protocol.DefaultParametersValueMaxSize * 1024 * 1024
	//	}
	//	v.log.Infof("[BlockVerifierImpl] receive msg, topic: %s, blockverify[%v]",
	//		msg.Topic.String(), v.chainConf.ChainConfig().Block)
	//default:
	//
	//}
}

func (v *DeterministicBlockVerifierImpl) OnQuit() {
	// nothing, implement Subscriber interface
}

// verifyBlockWithoutDag
// 1. follower need to add txs into pending cache
// 2. proposer return verify result directly
// 3. re calc block hash as final hash to commit
func (v *DeterministicBlockVerifierImpl) verifyBlockWithoutDag(block *commonpb.Block, mode protocol.VerifyMode) (
	verifyResult *consensuspb.VerifyResult, err error,
) {
	/**
	1. verify pre Block
	2. schedule pre block
	3. finalize pre block
	*/

	startTick := utils.CurrentTimeMillisSeconds()
	if err = utils.IsEmptyBlock(block); err != nil {
		v.log.Error(err)
		v.log.Debugf("empty block,height:%+v, hash:%+v, chainId:%+v, preHash:%x, signature:%+x",
			block.Header.BlockHeight, block.Header.BlockHash,
			block.Header.ChainId, block.Header.PreBlockHash, block.Header.Signature)
		return nil, err
	}

	v.log.Debugf("verify receive [%d](%x,%d,%d), from sync %d",
		block.Header.BlockHeight, block.Header.BlockHash, block.Header.TxCount, len(block.Txs), mode)
	// avoid concurrent verify, only one block hash can be verified at the same time
	if !v.reentrantLocks.Lock(string(block.Header.BlockHash)) {
		v.log.Warnf("block(%d,%x) concurrent verify, yield", block.Header.BlockHeight, block.Header.BlockHash)
		return nil, commonErrors.ErrConcurrentVerify
	}
	defer v.reentrantLocks.Unlock(string(block.Header.BlockHash))

	// get the pre hash to fil in verifyResult's msg
	preHash := block.Header.BlockHash

	// No duplicate verify
	isRepeat := v.verifyRepeat(block, mode)
	if isRepeat {
		elapsed := utils.CurrentTimeMillisSeconds() - startTick
		v.log.Infof("verify success repeat [%d](%x), total: %d. mode:%d",
			block.Header.BlockHeight, block.Header.BlockHash, elapsed, mode)
		return nil, nil
	}

	var contractEventMap map[string][]*commonpb.ContractEvent
	// avoid to recover the committed block.
	lastBlock, err := v.verifierBlock.FetchLastBlock(block)
	if err != nil {
		return nil, err
	}

	startPoolTick := utils.CurrentTimeMillisSeconds()
	newBlock, batchIds, err := common.RecoverBlock(block, mode, v.chainConf, v.txPool, v.ac, v.netService, v.log)
	if err != nil {
		v.log.Errorf("RecoverBlock failed, err:%v", err)
		return nil, err
	}
	lastPool := utils.CurrentTimeMillisSeconds() - startPoolTick

	// verify pre block(cap、tx count、preBlock)
	startBlockVerifyTick := utils.CurrentTimeMillisSeconds()
	_, _, _, _, err = v.validateBlock(newBlock, lastBlock, mode)
	if err != nil {
		v.log.Warnf("verify failed [%d](%x),preBlockHash:%x, %s",
			newBlock.Header.BlockHeight, newBlock.Header.BlockHash, newBlock.Header.PreBlockHash, err.Error())
		if protocol.CONSENSUS_VERIFY == mode {
			v.log.DebugDynamic(func() string {
				return fmt.Sprintf("publish verfiy failed rw set txs, block height:%d, err: %s",
					newBlock.Header.BlockHeight, err.Error())
			})
			v.msgBus.Publish(msgbus.VerifyResult, parseVerifyResult(
				newBlock, false, nil, nil))
		}

		return nil, err
	}
	blockVerifyUsed := utils.CurrentTimeMillisSeconds() - startBlockVerifyTick

	startSigTick := utils.CurrentTimeMillisSeconds()
	v.log.DebugDynamic(func() string {
		return fmt.Sprintf("verify block \n %s", utils.FormatBlock(newBlock))
	})
	sigUsed := utils.CurrentTimeMillisSeconds() - startSigTick

	snapshot := v.snapshotManager.NewSnapshot(lastBlock, newBlock)
	startVMTick := utils.CurrentTimeMillisSeconds()

	//// Determinism verification: clone inputs and run scheduler twice
	//blockClone := proto.Clone(newBlock).(*commonpb.Block)
	//validatedTxsClone := make([]*commonpb.Transaction, len(newBlock.Txs))
	//copy(validatedTxsClone, newBlock.Txs)
	//snapshotClone := v.snapshotManager.NewSnapshot(lastBlock, blockClone)

	// 主节点和从节点都在这里执行
	txRWSetMap, _, err := v.txScheduler.Schedule(newBlock, newBlock.Txs, snapshot)

	//// 算法的确定性验证：
	//_, _, err2 := v.txScheduler.Schedule(blockClone, validatedTxsClone, snapshotClone)
	//if err2 == nil && err == nil {
	//	// Compare newBlock.Txs with blockClone.Txs
	//	if len(newBlock.Txs) != len(blockClone.Txs) {
	//		v.log.Errorf("Scheduler determinism check FAILED: tx count mismatch, first=%d, second=%d",
	//			len(newBlock.Txs), len(blockClone.Txs))
	//	} else {
	//		allMatch := true
	//		for i := range newBlock.Txs {
	//			if newBlock.Txs[i].Payload.TxId != blockClone.Txs[i].Payload.TxId {
	//				allMatch = false
	//				v.log.Errorf("Scheduler determinism check FAILED: tx order mismatch at index %d, first=%s, second=%s",
	//					i, newBlock.Txs[i].Payload.TxId, blockClone.Txs[i].Payload.TxId)
	//				break
	//			}
	//		}
	//		if allMatch {
	//			v.log.Infof("Scheduler determinism check PASSED: both runs produced identical tx list with %d txs", len(newBlock.Txs))
	//		}
	//	}
	//}

	vmUsed := utils.CurrentTimeMillisSeconds() - startVMTick

	if !utils.CanProposeEmptyBlock(v.chainConf.ChainConfig().Consensus.Type) && len(newBlock.Txs) == 0 {
		return nil, fmt.Errorf("no txs in scheduled block, proposing block ends")
	}

	// finalize block
	finalizeStartTick := utils.CurrentTimeMillisSeconds()
	err = common.FinalizeBlock(
		newBlock,
		txRWSetMap,
		nil,
		v.chainConf.ChainConfig().Crypto.Hash,
		true,
		v.log)
	finalizeLasts := utils.CurrentTimeMillisSeconds() - finalizeStartTick
	if err != nil {
		return nil, fmt.Errorf("finalizeBlock block(%d,%s) error %s",
			newBlock.Header.BlockHeight, hex.EncodeToString(newBlock.Header.BlockHash), err)
	}

	// re calc block hash(final hash)
	blockHash, err := utils.CalcBlockHash(v.chainConf.ChainConfig().Crypto.Hash, newBlock)
	if err != nil {
		return nil, err
	}

	newBlock.Header.BlockHash = blockHash

	// normal 交易池下，这里可能是空，需要初始化一下
	if newBlock.AdditionalData.ExtraData == nil {
		newBlock.AdditionalData.ExtraData = make(map[string][]byte)
	}

	// verify success, cache block and read write set
	// solo need this，too！！！
	v.log.Debugf("set proposed block(%d,%x)", newBlock.Header.BlockHeight, newBlock.Header.BlockHash)
	if err = v.proposalCache.SetProposedBlock(&protocol.ProposalData{
		Block:            newBlock,
		TxRwSetMap:       txRWSetMap,
		ContractEventMap: contractEventMap,
	}, true); err != nil {
		return nil, err
	}

	// mark transactions in block as pending status in tx pool(except proposer)
	if mode != protocol.PROPOSER_VERIFY {
		if common.TxPoolType == batch.TxPoolType {
			v.txPool.AddTxBatchesToPendingCache(batchIds, newBlock.Header.BlockHeight)
		} else {
			v.txPool.AddTxsToPendingCache(newBlock.Txs, newBlock.Header.BlockHeight)
		}
	}

	elapsed := utils.CurrentTimeMillisSeconds() - startTick
	// todo log optimize
	v.log.Infof("verify success [%d,%x]"+
		"(blockVerify:%d,sign:%d,vm:%d,finalize:%d,pool:%d,total:%d)",
		newBlock.Header.BlockHeight, newBlock.Header.BlockHash, blockVerifyUsed, sigUsed, vmUsed,
		finalizeLasts, lastPool, elapsed)

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		v.metricBlockVerifyTime.WithLabelValues(v.chainId).Observe(float64(elapsed) / 1000)
	}
	verifyResult = parseVerifyResult(newBlock, true, txRWSetMap, nil)
	if verifyResult.Code == consensuspb.VerifyResult_SUCCESS {
		verifyResult.Msg = string(preHash)
	}

	return verifyResult, nil
}

func (v *DeterministicBlockVerifierImpl) verifyBlock(block *commonpb.Block, mode protocol.VerifyMode) (err error) {
	blockVersion := block.Header.BlockVersion
	startTick := utils.CurrentTimeMillisSeconds()
	if err = utils.IsEmptyBlock(block); err != nil {
		v.log.Errorf("%+v, block: %+v", err, block)
		return err
	}

	v.log.Debugf("verify receive [%d](%x,%d,%d), from sync %d",
		block.Header.BlockHeight, block.Header.BlockHash, block.Header.TxCount, len(block.Txs), mode)
	// avoid concurrent verify, only one block hash can be verified at the same time
	if !v.reentrantLocks.Lock(string(block.Header.BlockHash)) {
		v.log.Warnf("block(%d,%x) concurrent verify, yield", block.Header.BlockHeight, block.Header.BlockHash)
		return commonErrors.ErrConcurrentVerify
	}
	defer v.reentrantLocks.Unlock(string(block.Header.BlockHash))

	// No duplicate verify
	isRepeat := v.verifyRepeat(block, mode)
	if isRepeat {
		elapsed := utils.CurrentTimeMillisSeconds() - startTick
		v.log.Infof("verify success repeat [%d](%x), total: %d. mode:%d",
			block.Header.BlockHeight, block.Header.BlockHash, elapsed, mode)
		return nil
	}

	// avoid to recover the committed block.
	lastBlock, err := v.verifierBlock.FetchLastBlock(block)
	if err != nil {
		return err
	}

	// startPoolTick := utils.CurrentTimeMillisSeconds()
	newBlock, batchIds, err := common.RecoverBlock(block, mode, v.chainConf, v.txPool, v.ac, v.netService, v.log)
	if err != nil {
		v.log.Errorf("RecoverBlock failed, err:%v", err)
		return err
	}
	// lastPool := utils.CurrentTimeMillisSeconds() - startPoolTick

	//var (
	//	txRWSetMap       map[string]*commonpb.TxRWSet
	//	contractEventMap map[string][]*commonpb.ContractEvent
	//	timeLasts         map[string]int64
	//	rwSetVerifyFailTx *common.RwSetVerifyFailTx
	//	//layer2TxsMap      map[string]*consensuspb.Layer2TxsSet
	//	//layer2TxRoots     map[string][]byte
	//)

	txRWSetMap, contractEventMap, _, rwSetVerifyFailTx, err := v.validateBlock(newBlock, lastBlock, mode)
	if err != nil {
		v.log.Warnf("verify failed [%d](%x),preBlockHash:%x, %s",
			newBlock.Header.BlockHeight, newBlock.Header.BlockHash, newBlock.Header.PreBlockHash, err.Error())
		if protocol.CONSENSUS_VERIFY == mode {
			v.log.DebugDynamic(func() string {
				return fmt.Sprintf("publish verfiy failed rw set txs, block height:%d, err: %s",
					newBlock.Header.BlockHeight, err.Error())
			})
			v.msgBus.Publish(msgbus.VerifyResult,
				parseVerifyResult(newBlock, false, txRWSetMap, rwSetVerifyFailTx))
		}

		// rollback sql
		if sqlErr := v.storeHelper.RollBack(newBlock, v.blockchainStore); sqlErr != nil {
			v.log.Errorf("block [%d] rollback sql failed: %s", newBlock.Header.BlockHeight, sqlErr)
		}

		// clear snapshot when verify fail
		if snapErr := v.snapshotManager.ClearSnapshot(newBlock); snapErr != nil {
			snapErr = fmt.Errorf("clear snapshot fail[%d](hash:%x), err: %s",
				block.Header.BlockHeight, block.Header.BlockHash, snapErr.Error())
			v.log.Error(snapErr)
		}

		return err
	}

	snapshot := v.snapshotManager.GetSnapshot(lastBlock, newBlock)
	if coinbasemgr.IsOptimizeChargeGasEnabled(v.chainConf) {
		if err = scheduler.VerifyOptimizeChargeGasTx(newBlock, snapshot, v.ac, blockVersion); err != nil {
			v.log.Warnf("verify failed [%d](%x), %s",
				newBlock.Header.BlockHeight, newBlock.Header.BlockHash, err.Error())
			if protocol.CONSENSUS_VERIFY == mode {
				v.msgBus.Publish(msgbus.VerifyResult,
					parseVerifyResult(newBlock, false, txRWSetMap, rwSetVerifyFailTx))
			}
			return err
		}
	}

	// sync mode, need to verify consensus vote signature
	// beginConsensCheck := utils.CurrentTimeMillisSeconds()
	if protocol.SYNC_VERIFY == mode {
		if err = v.verifyVoteSig(newBlock); err != nil {
			v.log.Warnf("verify failed [%d](%x), vote sig %s",
				newBlock.Header.BlockHeight, newBlock.Header.BlockHash, err.Error())
			return err
		}
	}
	// consensusCheckUsed := utils.CurrentTimeMillisSeconds() - beginConsensCheck

	// verify success, cache block and read write set
	// solo need this，too！！！
	v.log.Debugf("set proposed block(%d,%x)", newBlock.Header.BlockHeight, newBlock.Header.BlockHash)
	if err = v.proposalCache.SetProposedBlock(&protocol.ProposalData{
		Block:            newBlock,
		TxRwSetMap:       txRWSetMap,
		ContractEventMap: contractEventMap,
		// Layer2TxRoots:    layer2TxRoots,
	}, false); err != nil {
		return err
	}

	// follower update mlc tx set
	//fingerPrint := string(utils.CalcBlockFingerPrint(block))
	//if layer2TxsMap != nil {
	//	v.mlcCache.SetMLCTxSet(block.Header.BlockHeight, fingerPrint, layer2TxsMap)
	//}

	//layer2TxRootsToString := make(map[string]string)
	//for k, v := range layer2TxRoots {
	//	layer2TxRootsToString[k] = hex.EncodeToString(v)
	//}
	//v.log.Infof("block[height:%d,fingerPrint:%s] calc layer2 tx roots:%v",
	//	block.Header.BlockHeight, fingerPrint, layer2TxRootsToString)

	// mark transactions in block as pending status in txpool
	if common.TxPoolType == batch.TxPoolType {
		v.txPool.AddTxBatchesToPendingCache(batchIds, newBlock.Header.BlockHeight)
	} else {
		v.txPool.AddTxsToPendingCache(coinbasemgr.FilterCoinBaseTxOrGasTx(newBlock.Txs), newBlock.Header.BlockHeight)
	}

	if protocol.CONSENSUS_VERIFY == mode {
		v.msgBus.Publish(msgbus.VerifyResult,
			parseVerifyResult(newBlock, true, txRWSetMap, nil))
	}
	elapsed := utils.CurrentTimeMillisSeconds() - startTick
	// v.log.Infof("verify success [%d,%x]"+
	//	"(blockSig:%d,vm:%d,txVerify:%d,txRoot:%d,layer2TxRoot:%d,pool:%d,consensusCheckUsed:%d,total:%d)",
	//	newBlock.Header.BlockHeight, newBlock.Header.BlockHash, timeLasts[common.Time_Statistics_BlockSig],
	//	timeLasts[common.Time_Statistics_VM], timeLasts[common.Time_Statistics_TxVerify],
	//	timeLasts[common.Time_Statistics_TxRoot], timeLasts[common.Time_Statistics_Layer2TxRoot],
	//	lastPool, consensusCheckUsed, elapsed)

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		v.metricBlockVerifyTime.WithLabelValues(v.chainId).Observe(float64(elapsed) / 1000)
	}

	return nil
}

func (v *DeterministicBlockVerifierImpl) validateBlock(block, lastBlock *commonpb.Block, mode protocol.VerifyMode) (
	map[string]*commonpb.TxRWSet,
	map[string][]*commonpb.ContractEvent,
	map[string]int64,
	*common.RwSetVerifyFailTx, error,
) {
	hashType := v.chainConf.ChainConfig().Crypto.Hash
	timeLasts := make(map[string]int64)
	var err error
	var txCapacity uint32
	if coinbasemgr.IsOptimizeChargeGasEnabled(v.chainConf) {
		txCapacity = v.chainConf.ChainConfig().Block.BlockTxCapacity + 1
	} else {
		txCapacity = v.chainConf.ChainConfig().Block.BlockTxCapacity
	}
	if block.Header.TxCount > txCapacity {
		err = fmt.Errorf("txcapacity expect <= %d, got %d)", txCapacity, block.Header.TxCount)
		return nil, nil, timeLasts, nil, err
	}

	if err = common.IsTxCountValid(block); err != nil {
		return nil, nil, timeLasts, nil, err
	}

	// proposed height == proposing height - 1
	proposedHeight := lastBlock.Header.BlockHeight
	// check if this block height is 1 bigger than last block height
	lastBlockHash := lastBlock.Header.BlockHash
	err = common.CheckPreBlock(block, lastBlockHash, proposedHeight)
	if err != nil {
		return nil, nil, timeLasts, nil, err
	}

	v.log.DebugDynamic(func() string {
		return fmt.Sprintf("verify block for mode[%d] (%d-%x)", mode, block.Header.BlockHeight, block.Header.BlockHash)
	})

	if mode == protocol.SYNC_VERIFY {
		return v.verifierBlock.ValidateBlock(block, lastBlock, hashType, timeLasts, mode)
	}

	return v.verifierBlock.ValidateBlockWithoutSimulate(block, lastBlock, hashType, timeLasts, mode)
}

func (v *DeterministicBlockVerifierImpl) validateBlockWithRWSets(block, lastBlock *commonpb.Block, mode protocol.VerifyMode,
	txRWSetMap map[string]*commonpb.TxRWSet) (
	map[string][]*commonpb.ContractEvent, error,
) {
	hashType := v.chainConf.ChainConfig().Crypto.Hash
	var err error
	var txCapacity uint32
	if coinbasemgr.IsOptimizeChargeGasEnabled(v.chainConf) {
		txCapacity = v.chainConf.ChainConfig().Block.BlockTxCapacity + 1
	} else {
		txCapacity = v.chainConf.ChainConfig().Block.BlockTxCapacity
	}
	if block.Header.TxCount > txCapacity {
		return nil, fmt.Errorf("txcapacity expect <= %d, got %d)", txCapacity, block.Header.TxCount)
	}

	if err = common.IsTxCountValid(block); err != nil {
		return nil, err
	}

	// proposed height == proposing height - 1
	proposedHeight := lastBlock.Header.BlockHeight
	// check if this block height is 1 bigger than last block height
	lastBlockHash := lastBlock.Header.BlockHash
	err = common.CheckPreBlock(block, lastBlockHash, proposedHeight)
	if err != nil {
		return nil, err
	}

	return v.verifierBlock.ValidateBlockWithoutExecuting(block, hashType, txRWSetMap, mode)
}

func (v *DeterministicBlockVerifierImpl) verifyVoteSig(block *commonpb.Block) error {
	return consensus.VerifyBlockSignatures(v.chainConf, v.ac, v.blockchainStore, block, v.ledgerCache)
}

func (v *DeterministicBlockVerifierImpl) cutBlocks(blocksToCut []*commonpb.Block, blockToKeep *commonpb.Block) {
	if common.TxPoolType == batch.TxPoolType {
		err := v.cutBlocksForBatchPool(blocksToCut, blockToKeep)
		if err != nil {
			v.log.Warnf(fmt.Sprintf("cut block[%d] failed, err:%v", blockToKeep.Header.BlockHeight, err))
		}
		return
	}

	cutTxs := make([]*commonpb.Transaction, 0)
	txMap := make(map[string]interface{})
	for _, tx := range blockToKeep.Txs {
		txMap[tx.Payload.TxId] = struct{}{}
	}
	for _, blockToCut := range blocksToCut {
		v.log.Infof("cut block hash: %x, height: %v", blockToCut.Header.BlockHash, blockToCut.Header.BlockHeight)
		for _, txToCut := range blockToCut.Txs {
			if _, ok := txMap[txToCut.Payload.TxId]; ok {
				// this transaction is kept, do NOT cut it.
				continue
			}
			v.log.Debugf("cut tx hash: %s", txToCut.Payload.TxId)
			cutTxs = append(cutTxs, txToCut)
		}
	}
	if len(cutTxs) > 0 {
		common.RetryAndRemoveTxs(v.txPool, cutTxs, nil, v.log)
	}
}

func (v *DeterministicBlockVerifierImpl) cutBlocksForBatchPool(blocksToCut []*commonpb.Block, blockToKeep *commonpb.Block) error {
	keepBatchIdsMap := make(map[string]interface{})
	batchIds, _, err := common.GetBatchIds(blockToKeep)
	if err != nil {
		v.log.Errorf("get batch ids from keep block[%d,%x] failed, err:%v",
			blockToKeep.Header.BlockHeight, blockToKeep.Header.BlockHash, err)
		return err
	}
	for _, batchId := range batchIds {
		keepBatchIdsMap[batchId] = struct{}{}
	}

	finalCutBatchIds := make([]string, 0)
	for _, blockToCut := range blocksToCut {
		v.log.Infof("cut block hash: %x, height: %v", blockToCut.Header.BlockHash, blockToCut.Header.BlockHeight)
		cutBatchIds, _, err := common.GetBatchIds(blockToCut)
		if err != nil {
			v.log.Warnf("get batch ids from removed block[%d,%x] failed, err:%v",
				blockToCut.Header.BlockHeight, blockToCut.Header.BlockHash, err)
			continue
		}
		for _, cutBatchId := range cutBatchIds {
			if _, ok := keepBatchIdsMap[cutBatchId]; ok {
				// this transaction is kept, do NOT cut it.
				continue
			}
			v.log.Debugf("cut tx batchId: %s", cutBatchId)
			finalCutBatchIds = append(finalCutBatchIds, cutBatchId)
		}
	}

	if len(finalCutBatchIds) > 0 {
		v.txPool.RetryAndRemoveTxBatches(finalCutBatchIds, nil)
	}

	return nil
}

// verifyRepeat to check if the block has verified before
func (v *DeterministicBlockVerifierImpl) verifyRepeat(block *commonpb.Block,
	mode protocol.VerifyMode,
) (isRepeat bool) {
	proposalData := v.proposalCache.GetProposedBlock(block)
	// Return not repeat if SQL is not enabled or if it is not solo
	if proposalData == nil {
		return false
	}

	proposedBlock := proposalData.Block
	txRwSet := proposalData.TxRwSetMap
	isSqlDb := v.chainConf.ChainConfig().Contract.EnableSqlSupport

	// 确定性调度下，主节点存了proposalBlock，但是尚未执行，需要执行该block
	if mode == protocol.PROPOSER_VERIFY && len(txRwSet) == 0 {
		return false
	}

	if consensuspb.ConsensusType_SOLO != v.chainConf.ChainConfig().Consensus.Type || isSqlDb {
		// the block has verified before
		// 主、从节点cache中已执行过本block
		if protocol.CONSENSUS_VERIFY == mode {
			// consensus mode, publish verify result to message bus
			v.msgBus.Publish(msgbus.VerifyResult, parseVerifyResult(proposedBlock, true, txRwSet, nil))
		}
		lastBlock, _ := v.proposalCache.GetProposedBlockByHashAndHeight(
			block.Header.PreBlockHash, block.Header.BlockHeight-1)
		if lastBlock == nil {
			v.log.Debugf(
				"no pre-block be found, preHeight:%d, preBlockHash:%x",
				block.Header.BlockHeight-1,
				block.Header.PreBlockHash,
			)
			return true
		}
		cutBlocks := v.proposalCache.KeepProposedBlock(lastBlock.Header.BlockHash, lastBlock.Header.BlockHeight)
		if len(cutBlocks) > 0 {
			v.log.Infof(
				"received block hash: %s, height: %v",
				hex.EncodeToString(lastBlock.Header.BlockHash),
				lastBlock.Header.BlockHeight,
			)
			v.cutBlocks(cutBlocks, lastBlock)
		}

		return true
	}
	return false
}
