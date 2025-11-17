/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package proposer

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"chainmaker.org/chainmaker/common/v2/monitor"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/pb-go/v2/consensus/maxbft"
	txpoolpb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"chainmaker.org/chainmaker/protocol/v2"
	batch "chainmaker.org/chainmaker/txpool-batch/v2"

	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker-go/module/core/common"
	"chainmaker.org/chainmaker-go/module/core/common/coinbasemgr"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
)

// DeterministicBlockProposerImpl implements BlockProposer interface.
// In charge of propose a new block.
type DeterministicBlockProposerImpl struct {
	chainId string // chain id, to identity this chain

	txPool          protocol.TxPool          // tx pool provides tx batch
	txScheduler     protocol.TxScheduler     // scheduler orders tx batch into DAG form and returns a block
	snapshotManager protocol.SnapshotManager // snapshot manager
	identity        protocol.SigningMember   // identity manager
	ledgerCache     protocol.LedgerCache     // ledger cache
	msgBus          msgbus.MessageBus        // channel to give out proposed block
	ac              protocol.AccessControlProvider
	blockchainStore protocol.BlockchainStore
	txFilter        protocol.TxFilter // Verify the transaction rules with TxFilter

	isProposer   bool        // whether current node can propose block now
	idle         bool        // whether current node is proposing or not
	proposeTimer *time.Timer // timer controls the proposing periods

	canProposeC   chan bool                   // channel to handle propose status change from consensus module
	txPoolSignalC chan *txpoolpb.TxPoolSignal // channel to handle propose signal from tx pool
	exitC         chan bool                   // channel to stop proposing loop
	proposalCache protocol.ProposalCache

	chainConf protocol.ChainConf // chain config

	idleMu         sync.Mutex   // for proposeBlock reentrant lock
	statusMu       sync.Mutex   // for propose status change lock
	proposerMu     sync.RWMutex // for isProposer lock, avoid race
	log            protocol.Logger
	finishProposeC chan bool // channel to receive signal to yield propose block

	metricBlockPackageTime *prometheus.HistogramVec
	metricRandomAttackTime *prometheus.CounterVec
	proposer               *pbac.Member

	blockBuilder *common.BlockBuilder
	storeHelper  conf.StoreHelper
}

func NewDeterministicBlockProposer(config BlockProposerConfig, log protocol.Logger) (protocol.BlockProposer, error) {
	dBPImpl := &DeterministicBlockProposerImpl{
		chainId:         config.ChainId,
		isProposer:      false, // not proposer when initialized
		idle:            true,
		msgBus:          config.MsgBus,
		blockchainStore: config.BlockchainStore,
		canProposeC:     make(chan bool),
		txPoolSignalC:   make(chan *txpoolpb.TxPoolSignal),
		exitC:           make(chan bool),
		txPool:          config.TxPool,
		snapshotManager: config.SnapshotManager,
		txScheduler:     config.TxScheduler,
		identity:        config.Identity,
		ledgerCache:     config.LedgerCache,
		proposalCache:   config.ProposalCache,
		chainConf:       config.ChainConf,
		ac:              config.AC,
		log:             log,
		finishProposeC:  make(chan bool),
		storeHelper:     config.StoreHelper,
		txFilter:        config.TxFilter,
	}

	var err error
	dBPImpl.proposer, err = dBPImpl.identity.GetMember()
	if err != nil {
		dBPImpl.log.Warnf("identity serialize failed, %s", err)
		return nil, err
	}

	// start propose timer
	dBPImpl.proposeTimer = time.NewTimer(getDuration(dBPImpl.chainConf))
	if !dBPImpl.isSelfProposer() {
		dBPImpl.proposeTimer.Stop()
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		dBPImpl.metricBlockPackageTime = monitor.NewHistogramVec(
			monitor.SUBSYSTEM_CORE_PROPOSER,
			"metric_block_package_time",
			"block package time metric",
			[]float64{0.005, 0.01, 0.015, 0.05, 0.1, 1, 2, 5, 10},
			"chainId",
		)

		//chainId,height,txInfo,timeStamp
		dBPImpl.metricRandomAttackTime = monitor.NewCounterVec(monitor.SUBSYSTEM_CORE_PROPOSER,
			"metric_random_tx_attack",
			"Total number of random tx attack",
			"chainId", "contractName", "method", "timeStamp")
	}

	dBPImpl.storeHelper = config.StoreHelper
	bbConf := &common.BlockBuilderConf{
		ChainId:         dBPImpl.chainId,
		TxPool:          dBPImpl.txPool,
		TxScheduler:     dBPImpl.txScheduler,
		SnapshotManager: dBPImpl.snapshotManager,
		Identity:        dBPImpl.identity,
		LedgerCache:     dBPImpl.ledgerCache,
		ProposalCache:   dBPImpl.proposalCache,
		ChainConf:       dBPImpl.chainConf,
		Log:             dBPImpl.log,
		StoreHelper:     config.StoreHelper,
	}

	dBPImpl.blockBuilder = common.NewBlockBuilder(bbConf)

	return dBPImpl, nil
}

// Start, start proposer
func (bp *DeterministicBlockProposerImpl) Start() error {
	defer bp.log.Info("block proposer starts")

	go bp.startProposingLoop()

	return nil
}

// Stop, stop proposing loop
func (bp *DeterministicBlockProposerImpl) Stop() error {
	defer bp.log.Infof("block proposer stopped")
	bp.exitC <- true
	return nil
}

// Start, start proposing loop
func (bp *DeterministicBlockProposerImpl) startProposingLoop() {
	for {
		select {
		case <-bp.proposeTimer.C:
			if !bp.isSelfProposer() {
				break
			}
			go bp.proposeBlock()

		case signal := <-bp.txPoolSignalC:
			if !bp.isSelfProposer() {
				break
			}
			if signal.SignalType != txpoolpb.SignalType_BLOCK_PROPOSE {
				break
			}
			go bp.proposeBlock()

		case <-bp.exitC:
			bp.proposeTimer.Stop()
			bp.log.Info("block proposer loop stopped")
			return
		}
	}
}

/*
 * shouldProposeByBFT, check if node should propose new block
 * Only for *BFT consensus
 * if node is proposer, and node is not propose right now, and last proposed block is committed, then return true
 */
func (bp *DeterministicBlockProposerImpl) shouldProposeByBFT(height uint64) bool {
	if !bp.isIdle() {
		// concurrent control, proposer is proposing now
		bp.log.Debugf("proposer is busy, not propose [%d] ", height)
		return false
	}
	committedBlock := bp.ledgerCache.GetLastCommittedBlock()
	if committedBlock == nil {
		bp.log.Errorf("no committed block found")
		return false
	}
	currentHeight := committedBlock.Header.BlockHeight
	// proposing height must higher than current height
	return currentHeight+1 == height
}

// proposeBlock, to check if proposer can propose block right now
// if so, start proposing
func (bp *DeterministicBlockProposerImpl) proposeBlock() {
	defer func() {
		if bp.isSelfProposer() {
			bp.proposeTimer.Reset(getDuration(bp.chainConf))
		}
	}()
	lastBlock := bp.ledgerCache.GetLastCommittedBlock()
	if lastBlock == nil {
		bp.log.Errorf("no committed block found")
		return
	}

	proposingHeight := lastBlock.Header.BlockHeight + 1
	if !bp.shouldProposeByBFT(proposingHeight) {
		return
	}
	if !bp.isIdle() {
		// concurrent control, proposer is proposing now
		bp.log.DebugDynamic(func() string {
			return fmt.Sprintf("proposer is busy, not propose [%d] ", proposingHeight)
		})
		return
	}
	if !bp.setNotIdle() {
		bp.log.Infof("concurrent propose block [%d], yield!", proposingHeight)
		return
	}
	defer bp.setIdle()

	go bp.proposing(proposingHeight, lastBlock.Header.BlockHash)
	// #DEBUG MODE#
	if localconf.ChainMakerConfig.DebugConfig.IsHaltPropose {
		go func() {
			bp.OnReceiveYieldProposeSignal(true)
		}()
	}

	<-bp.finishProposeC
}

// proposeFromProposalCache, propose from proposal cache
/**
1. fetch transactions from txPool if not enabled Multi-layered Consensus.
2. Retrieve previously validated block from cache based on height
    a. Since blocks in the cache are appended, use the latest cache.
3. Trigger eCNY transaction pruning on new round
    a. If there are no layer2 transactions in additionalData,
follow the original logic: fetch transactions from txPool, package, and execute the process.
    b. If the blocks in the cache consist entirely of layer2 transactions, do not perform pruning.
4. After removing layer2 transactions, recalculate the corresponding rwSetMap, contractEventMap, DAG, additionalData.
5. Return the block to the consensus.
*/

func (bp *DeterministicBlockProposerImpl) proposeFromProposalCache(height uint64, preHash []byte) (bool, *commonpb.Block) {

	// 优先从自己提案的cache中取区块
	selfProposedBlock := bp.proposalCache.GetSelfProposedBlockAt(height)
	if selfProposedBlock != nil {
		if needPropose := bp.dealProposalRequestWithProposalCache(height, selfProposedBlock, preHash); !needPropose {
			// not need to propose from txPool and no need to generate new block
			return false, nil
		}
	}

	// need to propose from txPool and generate new block
	return true, nil
}

func CopyHeader(blk *commonpb.Block, proposer *pbac.Member) *commonpb.BlockHeader {
	return &commonpb.BlockHeader{
		BlockVersion:   blk.Header.BlockVersion,
		BlockType:      blk.Header.BlockType,
		ChainId:        blk.Header.ChainId,
		BlockHeight:    blk.Header.BlockHeight,
		BlockHash:      nil,
		PreBlockHash:   blk.Header.PreBlockHash,
		PreConfHeight:  blk.Header.PreConfHeight,
		TxCount:        blk.Header.TxCount,
		TxRoot:         blk.Header.TxRoot,
		DagHash:        blk.Header.DagHash,
		RwSetRoot:      blk.Header.RwSetRoot,
		BlockTimestamp: blk.Header.BlockTimestamp,
		ConsensusArgs:  blk.Header.ConsensusArgs,
		Proposer:       proposer,
		Signature:      nil,
	}
}

func (bp *DeterministicBlockProposerImpl) proposeFromTxPool(height uint64, preHash []byte) *commonpb.Block {
	var (
		//fetchLasts          int64
		//filterValidateLasts int64
		//fetchTotalLasts     int64 // The total time consuming
		totalTimes   int64 // loop count
		fetchBatch   []*commonpb.Transaction
		batchIds     []string
		fetchBatches [][]*commonpb.Transaction // record the order about transaction in tx pool
	)
	// 根据TxFilter时间规则过滤交易，如果剩余的交易为0，则再次从交易池拉取交易，重复执行
	// The transaction is filtered according to txFilter time rule. If the remaining transaction is 0, the transaction
	// is pulled from the trading pool again and executed repeatedly
	//fetchTotalFirst := utils.CurrentTimeMillisSeconds()
	for {
		totalTimes++
		// retrieve tx batch from tx pool
		//fetchFirst := utils.CurrentTimeMillisSeconds()
		batchIds, fetchBatch, fetchBatches = bp.getFetchBatchFromPool(height)
		//proposeTimeCost[common.Time_Statistics_Fetch] += utils.CurrentTimeMillisSeconds() - fetchFirst
		bp.log.DebugDynamic(filtercommon.LoggingFixLengthFunc("begin proposing block[%d], fetch tx num[%d]",
			height, len(fetchBatch)))
		if len(fetchBatch) == 0 {
			bp.log.DebugDynamic(filtercommon.LoggingFixLengthFunc("no txs in tx pool, proposing block stoped"))
			return nil
		}
		// validate txFilter rules
		//filterValidateFirst := utils.CurrentTimeMillisSeconds()
		removeTxs, remainTxs := common.ValidateTxRules(bp.txFilter, fetchBatch)
		//proposeTimeCost[common.Time_Statistics_Filter] += utils.CurrentTimeMillisSeconds() - filterValidateFirst
		if len(removeTxs) > 0 {
			batchIds, fetchBatches, fetchBatch =
				bp.removeTx(height, batchIds, removeTxs, fetchBatch, fetchBatches)

			bp.log.Warnf("remove the overtime transactions, total:%d, remain:%d, remove:%d",
				len(fetchBatch), len(remainTxs), len(removeTxs))
		}
		if len(remainTxs) > 0 {
			// 剩余交易大于0则跳出循环
			fetchBatch = remainTxs
			break
		}
	}
	//proposeTimeCost[common.Time_Statistics_FetchTimes] = totalTimes
	//proposeTimeCost[common.Time_Statistics_FetchTotal] = utils.CurrentTimeMillisSeconds() - fetchTotalFirst

	if !utils.CanProposeEmptyBlock(bp.chainConf.ChainConfig().Consensus.Type) && len(fetchBatch) == 0 {
		// can not propose empty block and tx batch is empty, then yield proposing.
		bp.log.Debugf("no txs in tx pool, proposing block stopped")
		return nil
	}

	txCapacity := int(bp.chainConf.ChainConfig().Block.BlockTxCapacity)
	if len(fetchBatch) > txCapacity {
		// check if checkedBatch > txCapacity, if so, strict block tx count according to  config,
		// and put other txs back to txpool.
		txRetry := fetchBatch[txCapacity:]
		fetchBatch = fetchBatch[:txCapacity]

		if common.TxPoolType != batch.TxPoolType {
			common.RetryAndRemoveTxs(bp.txPool, txRetry, nil, bp.log)
		} else {
			batchIds, fetchBatches = bp.txPool.ReGenTxBatchesWithRetryTxs(height, batchIds,
				coinbasemgr.FilterCoinBaseTxOrGasTx(fetchBatch))
			fetchBatch = getFetchBatch(fetchBatches)
		}

		bp.log.Warnf("txbatch oversize expect <= %d, got %d", txCapacity, len(fetchBatch))
	}

	block, err := bp.generateNewBlock(height, preHash, fetchBatch,
		batchIds, fetchBatches)
	if err != nil {
		if common.TxPoolType != batch.TxPoolType {
			common.RetryAndRemoveTxs(bp.txPool, fetchBatch,
				nil, bp.log) // put txs back to txpool
		} else {
			bp.txPool.RetryAndRemoveTxBatches(batchIds, nil)
		}

		bp.log.Warnf("generate new block failed, %s", err.Error())
		return nil
	}

	return block
}

// proposing, propose a block in new height
func (bp *DeterministicBlockProposerImpl) proposing(height uint64, preHash []byte) *commonpb.Block {
	//startTick := utils.CurrentTimeMillisSeconds()
	defer bp.yieldProposing()
	//proposeTimeCost := common.GetProposeTimeStatistics()
	needProposeFromTxPool, block := bp.proposeFromProposalCache(height, preHash)
	if !needProposeFromTxPool && block == nil {
		return nil
	}

	if needProposeFromTxPool {
		block = bp.proposeFromTxPool(height, preHash)
	}

	if block == nil {
		return nil
	}

	proposalCache := bp.proposalCache.GetProposedBlock(block)
	bp.log.DebugDynamic(func() string {
		return fmt.Sprintf("proposing block \n %s", utils.FormatBlock(block))
	})

	// 可能此时proposalCache为空
	if proposalCache == nil {
		bp.log.Infof("proposer get proposal cache is nil at height %d", block.Header.BlockHeight)
		return nil
	}

	cutBlock := getCutBlock(bp.chainConf, block, bp.log)
	bp.msgBus.Publish(msgbus.ProposedBlock,
		&consensuspb.ProposalBlock{
			Block:    block,
			TxsRwSet: proposalCache.TxRwSetMap,
			CutBlock: cutBlock})
	//Layer2TxRoots: proposalCache.Layer2TxRoots}) // todo layer2 gow to deal ?

	//proposeTimeCost[common.Time_Statistics_TotalCost] = utils.CurrentTimeMillisSeconds() - startTick
	//bp.log.Infof("proposer success [%d](txs:%d),fetch(times:%v,fetch:%v,filter:%v,total:%d), time used("+
	//	"begin DB transaction:%v, new snapshot:%v, vm:%v, finalize block:%v,total:%d)",
	//	block.Header.BlockHeight, block.Header.TxCount,
	//	proposeTimeCost[common.Time_Statistics_FetchTimes], proposeTimeCost[common.Time_Statistics_Fetch],
	//	proposeTimeCost[common.Time_Statistics_Filter], proposeTimeCost[common.Time_Statistics_FetchTotal],
	//	proposeTimeCost[common.Time_Statistics_BeginDBTransaction], proposeTimeCost[common.Time_Statistics_NewSnapshot],
	//	proposeTimeCost[common.Time_Statistics_VM], proposeTimeCost[common.Time_Statistics_Finalize],
	//	proposeTimeCost[common.Time_Statistics_TotalCost])
	//
	//if localconf.ChainMakerConfig.MonitorConfig.Enabled {
	//	bp.metricBlockPackageTime.WithLabelValues(bp.chainId).Observe(
	//		float64(proposeTimeCost[common.Time_Statistics_TotalCost]) / 1000)
	//}

	return block
}

// OnReceiveTxPoolSignal, receive txpool signal and deliver to chan txpool signal
func (bp *DeterministicBlockProposerImpl) OnReceiveTxPoolSignal(txPoolSignal *txpoolpb.TxPoolSignal) {
	bp.txPoolSignalC <- txPoolSignal
}

/*
 * OnReceiveProposeStatusChange, to update isProposer status when received proposeStatus from consensus
 * if node is proposer, then reset the timer, otherwise stop the timer
 */
func (bp *DeterministicBlockProposerImpl) OnReceiveProposeStatusChange(proposeStatus bool) {
	bp.log.Debugf("OnReceiveProposeStatusChange(%t)", proposeStatus)
	bp.statusMu.Lock()
	defer bp.statusMu.Unlock()
	if proposeStatus == bp.isSelfProposer() {
		// 状态一致，忽略
		return
	}
	height, _ := bp.ledgerCache.CurrentHeight()
	bp.proposalCache.ResetProposedAt(height + 1) // proposer status changed, reset this round proposed status
	bp.setIsSelfProposer(proposeStatus)
	if !bp.isSelfProposer() {
		//bp.yieldProposing() // try to yield if proposer self is proposing right now.
		bp.log.Debug("current node is not proposer ")
		return
	}
	bp.proposeTimer.Reset(getDuration(bp.chainConf))
	bp.log.Debugf("current node is proposer, timeout period is %v", getDuration(bp.chainConf))
}

// OnReceiveMaxBFTProposal, to check if this proposer should propose a new block
// Only for maxbft consensus
func (bp *DeterministicBlockProposerImpl) OnReceiveMaxBFTProposal(proposal *maxbft.BuildProposal) {

}

// OnReceiveYieldProposeSignal, receive yield propose signal
func (bp *DeterministicBlockProposerImpl) OnReceiveYieldProposeSignal(isYield bool) {
	if !isYield {
		return
	}
	if bp.yieldProposing() {
		// halt scheduler execution
		bp.txScheduler.Halt()
		height, _ := bp.ledgerCache.CurrentHeight()
		bp.proposalCache.ResetProposedAt(height + 1)
	}
}

// OnReceiveConsensusFailTxSet solve consensus fail tx set
func (bp *DeterministicBlockProposerImpl) OnReceiveRwSetVerifyFailTxs(rwSetVerifyFailTxs *consensuspb.RwSetVerifyFailTxs) {

	//// log记录当前高度、hash、交易列表、分层共识集合
	//bp.log.Infof("deal consensus fail txSet:%v", consensusFailTxSet)
	//
	//if consensusFailTxSet == nil {
	//	return
	//}
	//
	//height := consensusFailTxSet.BlockHeight
	//hash := consensusFailTxSet.BlockHash
	//
	//// 将共识不通过的分层共识集合记录在mlcCache中 (commit时剔除)
	//block, _ := bp.proposalCache.GetProposedBlockByHashAndHeight(hash, height)
	//// core verify失败时，此处会为空，只需走正常随机函数剔除逻辑即可
	//var fingerPrint string
	//if block != nil && len(consensusFailTxSet.MlcFailSet) != 0 {
	//	fingerPrint = string(utils.CalcBlockFingerPrint(block))
	//	bp.mlcCache.SetMLCConsensusFailSet(
	//		height, fingerPrint, consensusFailTxSet.MlcFailSet)
	//
	//	bp.log.Warnf("set mlc consensus fail set,fingerPrint[%s], consensusFailTxSet[%v]",
	//		fingerPrint, consensusFailTxSet)
	//}
	//
	//// 删除读写集不一致的交易
	//bp.deleteRWSetFailTxs(height, fingerPrint, block, consensusFailTxSet)

	if common.TxPoolType == batch.TxPoolType {
		bp.log.Warnf("batch tx pool not support recover the problem about rwSet in conformity")
		return
	}
	height := rwSetVerifyFailTxs.BlockHeight
	block := bp.proposalCache.GetSelfProposedBlockAt(height)

	if block == nil {
		txsRet, _ := bp.txPool.GetTxsByTxIds(rwSetVerifyFailTxs.TxIds)
		txs := make([]*commonpb.Transaction, 0)
		for _, v := range txsRet {
			txs = append(txs, v)
		}
		common.RetryAndRemoveTxs(bp.txPool, nil, txs, bp.log)

		for _, tx := range txs {
			bp.log.Warnf("<METRIC> delete random Tx,chainId:%s, height:%d, "+
				"txId:%s, contractName:%s, method:%s, timeStamp:%d",
				bp.chainId, rwSetVerifyFailTxs.BlockHeight,
				tx.Payload.TxId, tx.Payload.ContractName, tx.Payload.Method, utils.CurrentTimeMillisSeconds())

			if localconf.ChainMakerConfig.MonitorConfig.Enabled {
				bp.metricRandomAttackTime.WithLabelValues(bp.chainId, tx.Payload.ContractName,
					tx.Payload.Method, getCurrentTimeHour()).Inc()
			}
		}
		return
	}

	retryTxs := make([]*commonpb.Transaction, 0, len(block.Txs))
	removeTxs := make([]*commonpb.Transaction, 0, len(block.Txs))
	txsMap := make(map[string]*commonpb.Transaction, len(block.Txs))
	for _, tx := range block.Txs {
		for _, txId := range rwSetVerifyFailTxs.TxIds {
			if tx.Payload.TxId == txId {
				txsMap[txId] = tx
				removeTxs = append(removeTxs, tx)
				break
			}
		}
	}

	for _, tx := range block.Txs {
		if _, ok := txsMap[tx.Payload.TxId]; !ok {
			retryTxs = append(retryTxs, tx)
		}
	}

	common.RetryAndRemoveTxs(bp.txPool, retryTxs,
		removeTxs, bp.log)
	bp.proposalCache.ClearProposedBlockAt(height)

	for _, tx := range removeTxs {
		bp.log.Warnf("<METRIC> delete random Tx,chainId:%s, height:%d, "+
			"txId:%s, contractName:%s, method:%s, timeStamp:%d",
			bp.chainId, rwSetVerifyFailTxs.BlockHeight,
			tx.Payload.TxId, tx.Payload.ContractName, tx.Payload.Method, utils.CurrentTimeMillisSeconds())

		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			bp.metricRandomAttackTime.WithLabelValues(bp.chainId, tx.Payload.ContractName,
				tx.Payload.Method, getCurrentTimeHour()).Inc()
		}
	}
}

func (bp *DeterministicBlockProposerImpl) deleteRWSetFailTxsWithNormalTxs(height uint64, txIds []string) {
	txsRet, _ := bp.txPool.GetTxsByTxIds(txIds)
	txs := make([]*commonpb.Transaction, 0)
	for _, v := range txsRet {
		txs = append(txs, v)
	}
	common.RetryAndRemoveTxs(bp.txPool, nil, txs, bp.log)

	for _, tx := range txs {
		bp.log.Warnf("<METRIC> delete random Tx,chainId:%s, height:%d, "+
			"txId:%s, contractName:%s, method:%s, timeStamp:%d",
			bp.chainId, height,
			tx.Payload.TxId, tx.Payload.ContractName, tx.Payload.Method, utils.CurrentTimeMillisSeconds())

		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			bp.metricRandomAttackTime.WithLabelValues(
				bp.chainId, tx.Payload.ContractName, tx.Payload.Method, getCurrentTimeHour()).Inc()
		}
	}

	bp.proposalCache.ClearProposedBlockAt(height)
}

// yieldProposing, to yield proposing handle
func (bp *DeterministicBlockProposerImpl) yieldProposing() bool {
	// signal finish propose only if proposer is not idle
	bp.idleMu.Lock()
	defer bp.idleMu.Unlock()
	if !bp.idle {
		bp.finishProposeC <- true
		//bp.idle = true
		return true
	}
	return false
}

// getChainVersion, get chain version from config.
// If not access from config, use default value.
// @Deprecated
// nolint: unused
func (bp *DeterministicBlockProposerImpl) getChainVersion() []byte {
	if bp.chainConf == nil || bp.chainConf.ChainConfig() == nil {
		return []byte(DEFAULTVERSION)
	}
	return []byte(bp.chainConf.ChainConfig().Version)
}

// setNotIdle, set not idle status
func (bp *DeterministicBlockProposerImpl) setNotIdle() bool {
	bp.idleMu.Lock()
	defer bp.idleMu.Unlock()
	if bp.idle {
		bp.idle = false
		return true
	}
	return false
}

// isIdle, to check if proposer is idle
func (bp *DeterministicBlockProposerImpl) isIdle() bool {
	bp.idleMu.Lock()
	defer bp.idleMu.Unlock()
	return bp.idle
}

// setIdle, set idle status
func (bp *DeterministicBlockProposerImpl) setIdle() {
	bp.idleMu.Lock()
	defer bp.idleMu.Unlock()
	bp.idle = true
}

// setIsSelfProposer, set isProposer status of this node
func (bp *DeterministicBlockProposerImpl) setIsSelfProposer(isSelfProposer bool) {
	bp.proposerMu.Lock()
	defer bp.proposerMu.Unlock()
	bp.isProposer = isSelfProposer
	if !bp.isProposer {
		bp.proposeTimer.Stop()
	} else {
		bp.proposeTimer.Reset(getDuration(bp.chainConf))
	}
}

// isSelfProposer, return if this node is consensus proposer
func (bp *DeterministicBlockProposerImpl) isSelfProposer() bool {
	bp.proposerMu.RLock()
	defer bp.proposerMu.RUnlock()
	return bp.isProposer
}

func (bp *DeterministicBlockProposerImpl) ProposeBlock(proposal *maxbft.BuildProposal) (*consensuspb.ProposalBlock, error) {

	return nil, nil
}

func (bp *DeterministicBlockProposerImpl) getFetchBatchFromPool(
	height uint64) ([]string, []*commonpb.Transaction, [][]*commonpb.Transaction) {
	if common.TxPoolType == batch.TxPoolType {
		batchIds, fetchBatches := bp.txPool.FetchTxBatches(height)

		fetchBatch := getFetchBatch(fetchBatches)

		return batchIds, fetchBatch, fetchBatches
	}

	return nil, bp.txPool.FetchTxs(height), nil
}

func (bp *DeterministicBlockProposerImpl) removeTx(
	height uint64, batchIds []string, removeTxs, fetchBatch []*commonpb.Transaction,
	fetchBatches [][]*commonpb.Transaction) ([]string, [][]*commonpb.Transaction, []*commonpb.Transaction) {
	// don't remove tx when is batchTx pool
	if common.TxPoolType == batch.TxPoolType {
		// remove and get new batchIds
		batchIds, fetchBatches = bp.txPool.ReGenTxBatchesWithRemoveTxs(height, batchIds, removeTxs)
		fetchBatch = getFetchBatch(fetchBatches)

		return batchIds, fetchBatches, fetchBatch
	}
	common.RetryAndRemoveTxs(bp.txPool, nil, removeTxs, bp.log)
	return batchIds, fetchBatches, fetchBatch
}

func (bp *DeterministicBlockProposerImpl) dealProposalRequestWithProposalCache(
	height uint64, selfProposedBlock *commonpb.Block, preHash []byte) (needPropose bool) {

	if bytes.Equal(selfProposedBlock.Header.PreBlockHash, preHash) {

		currentTime := utils.CurrentTimeSeconds()
		// when this block has some wrong tx and could not to reach an agreement.
		// we need to clear the old proposal cache when the old block's tx timeout.
		// we need to remove these txs from tx pool.
		if currentTime-selfProposedBlock.Header.BlockTimestamp >=
			int64(bp.chainConf.ChainConfig().Block.TxTimeout) {

			bp.proposalCache.ClearTheBlock(selfProposedBlock)
			if common.TxPoolType == batch.TxPoolType {
				batchIds, _, err := common.GetBatchIds(selfProposedBlock)
				if err != nil {
					// no need to handle this err,propose a new block.
					return true
				}
				bp.txPool.RetryAndRemoveTxBatches(nil, batchIds)
				return true
			}

			common.RetryAndRemoveTxs(bp.txPool, nil,
				coinbasemgr.FilterCoinBaseTxOrGasTx(selfProposedBlock.Txs), bp.log)
			return true
		}

		// when this block has some wrong could not to reach an agreement.
		// we want to verify block's timestamp.
		// we need to clear the old proposal cache when the old block time out.
		// we need to retry these txs into tx pool.
		if bp.chainConf.ChainConfig().Block.BlockTimestampVerify {
			if currentTime-selfProposedBlock.Header.BlockTimestamp >=
				int64(bp.chainConf.ChainConfig().Block.BlockTimeout) {

				bp.proposalCache.ClearTheBlock(selfProposedBlock)

				if common.TxPoolType == batch.TxPoolType {
					batchIds, _, err := common.GetBatchIds(selfProposedBlock)
					if err != nil {
						// no need to handle this err,propose a new block.
						return true
					}
					bp.txPool.RetryAndRemoveTxBatches(batchIds, nil)
					return true
				}

				common.RetryAndRemoveTxs(bp.txPool, selfProposedBlock.Txs,
					nil, bp.log)
				return true
			}
		}

		blockFinger := utils.CalcBlockFingerPrint(selfProposedBlock)
		lastProposeTime, err := getLastProposeTimeByBlockFinger(string(blockFinger))

		if err != nil {
			bp.log.Errorf("proposer fail, get last propose time by hash err %s", err.Error())
			return false
		}

		if lastProposeTime == 0 {
			return false
		}

		if utils.CurrentTimeMillisSeconds()-lastProposeTime >= 1000 {
			// Repeat propose block if node has proposed before at the same height
			bp.proposalCache.SetProposedAt(height)
			proposalData := bp.proposalCache.GetProposedBlock(selfProposedBlock)
			common.ProposeRepeatTimerMap.Store(string(blockFinger), utils.CurrentTimeMillisSeconds())

			cutBlock := new(commonpb.Block)
			if common.IfOpenConsensusMessageTurbo(bp.chainConf) ||
				common.TxPoolType == batch.TxPoolType {
				cutBlock = common.GetTurboBlock(selfProposedBlock, cutBlock, bp.chainConf, bp.log)
			} else {
				cutBlock = selfProposedBlock
			}

			bp.msgBus.Publish(msgbus.ProposedBlock, &consensuspb.ProposalBlock{Block: selfProposedBlock,
				TxsRwSet: proposalData.TxRwSetMap, CutBlock: cutBlock})
			bp.log.Infof("proposer success repeat [%d](txs:%d,hash:%x)",
				selfProposedBlock.Header.BlockHeight, selfProposedBlock.Header.TxCount,
				selfProposedBlock.Header.BlockHash)
		}
		return false

	}
	bp.proposalCache.ClearTheBlock(selfProposedBlock)
	// Note: It is not possible to re-add the transactions in the deleted block to txpool; because some
	// transactions may be included in other blocks to be confirmed, and it is impossible to quickly exclude
	// these pending transactions that have been entered into the block. Comprehensive considerations,
	// directly discard this block is the optimal choice. This processing method may only cause partial
	// transaction loss at the current node, but it can be solved by rebroadcasting on the client side.

	if common.TxPoolType == batch.TxPoolType {
		batchIds, _, err := common.GetBatchIds(selfProposedBlock)
		if err != nil {
			return true
		}
		bp.txPool.RetryAndRemoveTxBatches(nil, batchIds)
		return true
	}

	common.RetryAndRemoveTxs(bp.txPool, nil, selfProposedBlock.Txs, bp.log)

	return true
}
