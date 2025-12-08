package verifier

import (
	"fmt"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"chainmaker.org/chainmaker/common/v2/monitor"
	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2"
)

// BlockVerifierFactory implements the protocol.BlockVerifier interface, acting as a proxy to route to specific implementations based on configuration
type BlockVerifierFactory struct {
	// actualVerifier uses atomic.Value to store BlockVerifier interface
	// Supports concurrent reads (high frequency) and atomic updates (low frequency)
	actualVerifier        atomic.Value // stores BlockVerifier
	log                   protocol.Logger
	metricBlockVerifyTime *prometheus.HistogramVec // metrics monitor
}

// NewBlockVerifierFactory creates a BlockVerifierFactory instance
func NewBlockVerifierFactory(config BlockVerifierConfig, log protocol.Logger) (*BlockVerifierFactory, error) {
	factory := &BlockVerifierFactory{log: log}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		factory.metricBlockVerifyTime = monitor.NewHistogramVec(monitor.SUBSYSTEM_CORE_VERIFIER, "metric_block_verify_time",
			"block verify time metric", []float64{0.005, 0.01, 0.015, 0.05, 0.1, 1, 2, 5, 10}, "chainId")
	}
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
func (bvf *BlockVerifierFactory) createVerifier(config BlockVerifierConfig, log protocol.Logger) (BlockVerifier, error) {
	schedulerConf := config.ChainConf.ChainConfig().Scheduler

	if schedulerConf == nil {
		return NewBlockVerifier(config, log, bvf.metricBlockVerifyTime)
	}

	schedulerType := schedulerConf.ProcessType
	switch schedulerType {
	case configpb.ProcessType_EXECUTE_ON_PROPOSE:
		return NewBlockVerifier(config, log, bvf.metricBlockVerifyTime)
	case configpb.ProcessType_EXECUTE_AFTER_PROPOSE:
		return NewDeterministicBlockVerifier(config, log, bvf.metricBlockVerifyTime)
	default:
		return nil, fmt.Errorf("new block verifier failed, invalid scheduler type:%s", schedulerType)
	}
}

// getVerifier atomically loads the current verifier instance (lock-free, concurrency-safe)
func (bvf *BlockVerifierFactory) getVerifier() BlockVerifier {
	return bvf.actualVerifier.Load().(BlockVerifier)
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
	// 1. 获取旧 verifier (原子读取)
	oldVerifier, _ := bvf.actualVerifier.Load().(BlockVerifier)

	// 2. 创建新 verifier
	verifier, err := bvf.createVerifier(config, bvf.log)
	if err != nil {
		return err
	}

	// 3. 原子替换
	bvf.actualVerifier.Store(verifier)

	// 4. 清理旧 verifier - 从 MsgBus 取消订阅，释放资源
	if oldVerifier != nil {
		config.MsgBus.UnRegister(msgbus.ChainConfig, oldVerifier)
	}
	return nil
}
