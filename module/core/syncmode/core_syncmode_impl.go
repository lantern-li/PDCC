/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// syncmode means commit new block in sync way
package syncmode

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/gogo/protobuf/proto"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	chainConfConfig "chainmaker.org/chainmaker/pb-go/v2/config"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	tbftpb "chainmaker.org/chainmaker/pb-go/v2/consensus/tbft"
	txpoolpb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"chainmaker.org/chainmaker/protocol/v2"

	"chainmaker.org/chainmaker-go/module/core/common"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/utils"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
	"chainmaker.org/chainmaker-go/module/core/syncmode/proposer"
	"chainmaker.org/chainmaker-go/module/core/syncmode/verifier"
	"chainmaker.org/chainmaker-go/module/subscriber"
)

// CoreEngine is a block handle engine.
// One core engine for one chain.
// nolint: structcheck,unused
type CoreEngine struct {
	chainId   string             // chainId, identity of a chain
	chainConf protocol.ChainConf // chain config

	msgBus         msgbus.MessageBus              // message bus, transfer messages with other modules
	blockProposer  protocol.BlockProposer         // block proposer, to generate new block when node is proposer
	BlockVerifier  *verifier.BlockVerifierFactory // block verifier, to verify block that proposer generated
	BlockCommitter protocol.BlockCommitter        // block committer, to commit block to store after consensus
	txScheduler    protocol.TxScheduler           // transaction scheduler, schedule transactions run in vm
	MaxbftHelper   protocol.MaxbftHelper

	txPool          protocol.TxPool          // transaction pool, cache transactions to be pack in block
	vmMgr           protocol.VmManager       // vm manager
	blockchainStore protocol.BlockchainStore // blockchain store, to store block, transactions in DB
	snapshotManager protocol.SnapshotManager // snapshot manager, manage state data that not store yet

	quitC         <-chan interface{}          // quit chan, reserved for stop core engine running
	proposedCache protocol.ProposalCache      // cache proposed block and proposal status
	log           protocol.Logger             // logger
	subscriber    *subscriber.EventSubscriber // block subsriber

	netService       protocol.NetService
	ledgerCache      protocol.LedgerCache // for update scheduler、proposer、verifier、committer
	identity         protocol.SigningMember
	ac               protocol.AccessControlProvider
	txFilter         protocol.TxFilter
	storeHelper      conf.StoreHelper
	currentScheduler *chainConfConfig.SchedulerConfig // current scheduler config
	signer            protocol.SigningMember
}

// NewCoreEngine new a core engine.
func NewCoreEngine(cf *conf.CoreEngineConfig) (*CoreEngine, error) {
	core := &CoreEngine{
		chainId:          cf.ChainId,
		msgBus:           cf.MsgBus,
		txPool:           cf.TxPool,
		vmMgr:            cf.VmMgr,
		blockchainStore:  cf.BlockchainStore,
		snapshotManager:  cf.SnapshotManager,
		proposedCache:    cf.ProposalCache,
		chainConf:        cf.ChainConf,
		log:              cf.Log,
		netService:       cf.NetService,
		ledgerCache:      cf.LedgerCache,
		identity:         cf.Identity,
		ac:               cf.AC,
		txFilter:         cf.TxFilter,
		storeHelper:      cf.StoreHelper,
		quitC:            make(<-chan interface{}),
		currentScheduler: &chainConfConfig.SchedulerConfig{},
	}

	// "cf.ChainConf.ChainConfig().Scheduler == nil" means use non deterministic scheduler(SchedulerType = 0).
	if cf.ChainConf.ChainConfig().Scheduler != nil {
		core.currentScheduler = cf.ChainConf.ChainConfig().Scheduler
	}
	var err error
	core.signer, err = utils.InitSigner(cf.ChainConf.ChainConfig(), localconf.ChainMakerConfig)
	if err != nil {
		log.Fatalf("init signer of TxScheduler failed: err = %v", err)
	}

	// new a tx scheduler
	core.txScheduler = createTxScheduler(core)

	// Initialize the block proposer
	core.blockProposer, err = core.createBlockProposer()
	if err != nil {
		return nil, err
	}

	// Initialize the block verifier
	core.BlockVerifier, err = core.createBlockVerifier()
	if err != nil {
		return nil, err
	}

	// Initialize the block committer
	core.BlockCommitter, err = core.createBlockCommitter()
	if err != nil {
		return nil, err
	}

	// get the type of tx pool
	if value, ok := localconf.ChainMakerConfig.TxPoolConfig["pool_type"]; ok {
		common.TxPoolType, _ = value.(string)
		common.TxPoolType = strings.ToUpper(common.TxPoolType)
	}

	return core, nil
}

// createTxScheduler creates a transaction scheduler.
func createTxScheduler(c *CoreEngine) protocol.TxScheduler {
	var schedulerFactory scheduler.TxSchedulerFactory
	return schedulerFactory.NewTxScheduler(
		c.vmMgr, c.chainConf, c.storeHelper, c.ledgerCache, c.ac,c.signer)
}

// createBlockProposer initializes a block proposer.
func (c *CoreEngine) createBlockProposer() (protocol.BlockProposer, error) {
	// new a bock proposer
	proposerConfig := proposer.BlockProposerConfig{
		ChainId:         c.chainId,
		TxPool:          c.txPool,
		SnapshotManager: c.snapshotManager,
		MsgBus:          c.msgBus,
		Identity:        c.identity,
		LedgerCache:     c.ledgerCache,
		TxScheduler:     c.txScheduler,
		ProposalCache:   c.proposedCache,
		ChainConf:       c.chainConf,
		AC:              c.ac,
		BlockchainStore: c.blockchainStore,
		StoreHelper:     c.storeHelper,
		TxFilter:        c.txFilter,
	}

	pf := new(proposer.BlockProposerFactory)
	return pf.NewBlockProposer(proposerConfig, c.log)
}

// createBlockVerifier initializes a block verifier.
func (c *CoreEngine) createBlockVerifier() (*verifier.BlockVerifierFactory, error) {
	verifierConfig := verifier.BlockVerifierConfig{
		ChainId:         c.chainId,
		TxPool:          c.txPool,
		SnapshotManager: c.snapshotManager,
		MsgBus:          c.msgBus,
		LedgerCache:     c.ledgerCache,
		TxScheduler:     c.txScheduler,
		ProposedCache:   c.proposedCache,
		ChainConf:       c.chainConf,
		AC:              c.ac,
		BlockchainStore: c.blockchainStore,
		StoreHelper:     c.storeHelper,
		TxFilter:        c.txFilter,
		VmMgr:           c.vmMgr,
		NetService:      c.netService,
	}
	return verifier.NewBlockVerifierFactory(verifierConfig, c.log)
}

// createBlockVerifier update a block verifier.
func (c *CoreEngine) updateBlockVerifier() error {
	verifierConfig := verifier.BlockVerifierConfig{
		ChainId:         c.chainId,
		TxPool:          c.txPool,
		SnapshotManager: c.snapshotManager,
		MsgBus:          c.msgBus,
		LedgerCache:     c.ledgerCache,
		TxScheduler:     c.txScheduler,
		ProposedCache:   c.proposedCache,
		ChainConf:       c.chainConf,
		AC:              c.ac,
		BlockchainStore: c.blockchainStore,
		StoreHelper:     c.storeHelper,
		TxFilter:        c.txFilter,
		VmMgr:           c.vmMgr,
		NetService:      c.netService,
	}
	return c.BlockVerifier.UpdateBlockVerifier(verifierConfig)
}

// createBlockCommitter initializes a block committer.
func (c *CoreEngine) createBlockCommitter() (protocol.BlockCommitter, error) {
	committerConfig := common.BlockCommitterConfig{
		ChainId:         c.chainId,
		BlockchainStore: c.blockchainStore,
		SnapshotManager: c.snapshotManager,
		TxPool:          c.txPool,
		LedgerCache:     c.ledgerCache,
		ProposedCache:   c.proposedCache,
		ChainConf:       c.chainConf,
		MsgBus:          c.msgBus,
		Subscriber:      c.subscriber,
		Verifier:        c.BlockVerifier,
		StoreHelper:     c.storeHelper,
		TxFilter:        c.txFilter,
	}
	return common.NewBlockCommitter(committerConfig, c.log)
}

// OnQuit called when quit subsribe message from message bus
func (c *CoreEngine) OnQuit() {
	c.log.Info("on quit")
}

// OnMessage consume a message from message bus
func (c *CoreEngine) OnMessage(message *msgbus.Message) {
	// 1. receive proposal status from consensus
	// 2. receive verify block from consensus
	// 3. receive commit block message from consensus
	// 4. receive propose signal from txpool
	// 5. receive build proposal signal from maxbft consensus

	switch message.Topic {
	case msgbus.ProposeState:
		if proposeStatus, ok := message.Payload.(bool); ok {
			c.blockProposer.OnReceiveProposeStatusChange(proposeStatus)
		}
	case msgbus.VerifyBlock:
		go func() {
			if block, ok := message.Payload.(*commonpb.Block); ok {
				c.BlockVerifier.VerifyBlock(block, protocol.CONSENSUS_VERIFY) //nolint: errcheck
			}
		}()
	case msgbus.VerifyBlockWithRWSet:
		go func() {
			if proposal, ok := message.Payload.(*tbftpb.Proposal); ok {
				block := proposal.Block
				rwSetMap := proposal.TxsRwSet
				rwSets := make([]*commonpb.TxRWSet, len(rwSetMap))

				if rwSetMap == nil {
					c.BlockVerifier.VerifyBlock(block, protocol.CONSENSUS_VERIFY) //nolint: errcheck
				} else {
					for index, tx := range block.Txs {
						rwSets[index] = rwSetMap[tx.Payload.TxId]
					}
					c.BlockVerifier.VerifyBlockWithRwSets(block, rwSets, protocol.CONSENSUS_VERIFY) //nolint: errcheck
				}
			}
		}()
	case msgbus.CommitBlock:
		go func() {
			if block, ok := message.Payload.(*commonpb.Block); ok {
				if err := c.BlockCommitter.AddBlock(block); err != nil {
					c.log.Warnf("put block(%d,%x) error %s",
						block.Header.BlockHeight,
						block.Header.BlockHash,
						err.Error())
				}
			}
		}()
	case msgbus.TxPoolSignal:
		if signal, ok := message.Payload.(*txpoolpb.TxPoolSignal); ok {
			c.blockProposer.OnReceiveTxPoolSignal(signal)
		}
	case msgbus.ConsensusFailTxs:
		if signal, ok := message.Payload.(*consensuspb.RwSetVerifyFailTxs); ok {
			c.log.DebugDynamic(func() string {
				return fmt.Sprintf("received consensus rw set verify fail txs block height:%d", signal.BlockHeight)
			})
			c.blockProposer.OnReceiveRwSetVerifyFailTxs(signal)
		}

	case msgbus.ChainConfig:
		dataStr, ok := message.Payload.([]string)
		if !ok {
			return
		}
		dataBytes, err := hex.DecodeString(dataStr[0])
		if err != nil {
			c.log.Warn(err)
			return
		}
		chainConfig := &chainConfConfig.ChainConfig{}
		err = proto.Unmarshal(dataBytes, chainConfig)
		if err != nil {
			c.log.Warn(err)
			return
		}

		// update chain config
		c.updateChinConfig(chainConfig)

		c.log.Infof("[BlockVerifierImpl] receive msg, topic: %s, blockConfUpdate[%v], ScheduleConfUpdate[%v]",
			message.Topic.String(), c.chainConf.ChainConfig().Block, c.chainConf.ChainConfig().Scheduler)
	}
}

// Start, initialize core engine
func (c *CoreEngine) Start() {
	c.msgBus.Register(msgbus.ProposeState, c)
	c.msgBus.Register(msgbus.VerifyBlock, c)
	c.msgBus.Register(msgbus.VerifyBlockWithRWSet, c)
	c.msgBus.Register(msgbus.CommitBlock, c)
	c.msgBus.Register(msgbus.TxPoolSignal, c)
	c.msgBus.Register(msgbus.ConsensusFailTxs, c)
	c.msgBus.Register(msgbus.ChainConfig, c)
	// c.msgBus.Register(msgbus.BuildProposal, c)
	c.blockProposer.Start() //nolint: errcheck
}

// Stop, stop core engine
func (c *CoreEngine) Stop() {
	defer c.log.Infof("core stopped.")
	c.blockProposer.Stop() //nolint: errcheck
}

func (c *CoreEngine) GetBlockProposer() protocol.BlockProposer {
	return c.blockProposer
}

func (c *CoreEngine) GetBlockCommitter() protocol.BlockCommitter {
	return c.BlockCommitter
}

func (c *CoreEngine) GetBlockVerifier() protocol.BlockVerifier {
	return c.BlockVerifier
}

func (c *CoreEngine) GetMaxbftHelper() protocol.MaxbftHelper {
	return c.MaxbftHelper
}

func (c *CoreEngine) updateChinConfig(chainConfig *chainConfConfig.ChainConfig) {
	c.chainConf.ChainConfig().Block = chainConfig.Block
	c.chainConf.ChainConfig().Scheduler = chainConfig.Scheduler
	c.chainConf.ChainConfig().AuthType = strings.ToLower(chainConfig.AuthType) // avoid not change to lower when need to new singer

	// update tx parameters ' max length
	protocol.ParametersValueMaxLength = chainConfig.Block.TxParameterSize * 1024 * 1024
	if chainConfig.Block.TxParameterSize <= 0 {
		protocol.ParametersValueMaxLength = protocol.DefaultParametersValueMaxSize * 1024 * 1024
	}

	if chainConfig.Scheduler != nil && (*c.currentScheduler).ProcessType != (*chainConfig.Scheduler).ProcessType &&
		(*c.currentScheduler).AlgorithmType != (*chainConfig.Scheduler).AlgorithmType {
		c.log.Infof("scheduler type update, new:%+v", *chainConfig.Scheduler)
		var storeHelper conf.StoreHelper
		if c.chainConf.ChainConfig().Contract.EnableSqlSupport {
			storeHelper = common.NewSQLStoreHelper(c.chainConf.ChainConfig().ChainId)
		} else {
			storeHelper = common.NewKVStoreHelper(c.chainConf.ChainConfig().ChainId)
		}

		// update scheduler
		var schedulerFactory scheduler.TxSchedulerFactory
		c.txScheduler = schedulerFactory.NewTxScheduler(c.vmMgr, c.chainConf, storeHelper, c.ledgerCache, c.ac,c.signer)

		err := c.blockProposer.Stop()
		if err != nil {
			c.log.Errorf("proposer stop  failed: %v", err)
			c.log.Panicf("proposer stop  failed: %v", err)
			return
		}

		c.blockProposer, err = c.createBlockProposer()
		if err != nil {
			c.log.Errorf("update block proposer failed: %v", err)
			c.log.Panicf("update block proposer failed: %v", err)
			return
		}

		err = c.updateBlockVerifier()
		if err != nil {
			c.log.Errorf("update block verifier failed: %v", err)
			c.log.Panicf("update block verifier failed: %v", err)
			return
		}

		c.currentScheduler = chainConfig.Scheduler

		// start new proposer
		err = c.blockProposer.Start()
		if err != nil {
			c.log.Errorf("proposer start  failed: %v", err)
			c.log.Panicf("proposer start  failed: %v", err)
			return
		}
	}
}
