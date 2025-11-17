package verifier

import (
	"fmt"
	"sync/atomic"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2"
)

// BlockVerifierFactory implements the protocol.BlockVerifier interface, acting as a proxy to route to specific implementations based on configuration
type BlockVerifierFactory struct {
	// actualVerifier uses atomic.Value to store protocol.BlockVerifier
	// Supports concurrent reads (high frequency) and atomic updates (low frequency)
	actualVerifier atomic.Value
	log            protocol.Logger
}

// NewBlockVerifierFactory creates a BlockVerifierFactory instance
func NewBlockVerifierFactory(config BlockVerifierConfig, log protocol.Logger) (*BlockVerifierFactory, error) {
	factory := &BlockVerifierFactory{log: log}

	// Initialize the concrete verifier instance
	verifier, err := factory.createVerifier(config, log)
	if err != nil {
		return nil, err
	}

	// Atomically store the verifier instance
	factory.actualVerifier.Store(verifier)

	return factory, nil
}

// createVerifier creates a concrete BlockVerifier implementation based on configuration
func (bvf *BlockVerifierFactory) createVerifier(config BlockVerifierConfig, log protocol.Logger) (protocol.BlockVerifier, error) {
	schedulerConf := config.ChainConf.ChainConfig().Scheduler
	if schedulerConf == nil {
		return NewBlockVerifier(config, log)
	}

	schedulerType := schedulerConf.SchedulerType
	switch schedulerType {
	case configpb.SchedulerType_DAG:
		return NewBlockVerifier(config, log)
	case configpb.SchedulerType_DETERMINISTIC:
		return NewDeterministicBlockVerifier(config, log)
	default:
		return nil, fmt.Errorf("new block verifier failed, invalid scheduler type:%s", schedulerType)
	}
}

// getVerifier atomically reads the current verifier instance (lock-free, concurrency-safe)
func (bvf *BlockVerifierFactory) getVerifier() protocol.BlockVerifier {
	return bvf.actualVerifier.Load().(protocol.BlockVerifier)
}

// VerifyBlock implements the protocol.BlockVerifier interface
func (bvf *BlockVerifierFactory) VerifyBlock(block *commonpb.Block, mode protocol.VerifyMode) error {
	verifier := bvf.getVerifier()
	return verifier.VerifyBlock(block, mode)
}

// VerifyBlockSync implements the protocol.BlockVerifier interface
func (bvf *BlockVerifierFactory) VerifyBlockSync(block *commonpb.Block, mode protocol.VerifyMode) (*consensuspb.VerifyResult, error) {
	verifier := bvf.getVerifier()
	return verifier.VerifyBlockSync(block, mode)
}

// VerifyBlockWithRwSets implements the protocol.BlockVerifier interface
func (bvf *BlockVerifierFactory) VerifyBlockWithRwSets(block *commonpb.Block, rwsets []*commonpb.TxRWSet, mode protocol.VerifyMode) error {
	verifier := bvf.getVerifier()
	return verifier.VerifyBlockWithRwSets(block, rwsets, mode)
}

// UpdateBlockVerifier updates the verifier instance based on new configuration (atomic operation, concurrency-safe)
func (bvf *BlockVerifierFactory) UpdateBlockVerifier(config BlockVerifierConfig) error {
	verifier, err := bvf.createVerifier(config, bvf.log)
	if err != nil {
		return err
	}

	// Atomically replace the verifier instance
	bvf.actualVerifier.Store(verifier)
	return nil
}
