package proposer

import (
	"errors"
	"fmt"

	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
)

type BlockProposerFactory struct {
}

func (bpf *BlockProposerFactory) NewBlockProposer(config BlockProposerConfig, log protocol.Logger) (protocol.BlockProposer, error) {

	schedulerConf := config.ChainConf.ChainConfig().Scheduler
	if schedulerConf == nil {
		return NewBlockProposer(config, log)
	}

	schedulerType := schedulerConf.ProcessType
	switch schedulerType {
	case configpb.ProcessType_EXECUTE_ON_PROPOSE:
		return NewBlockProposer(config, log)
	case configpb.ProcessType_EXECUTE_AFTER_PROPOSE:
		return NewDeterministicBlockProposer(config, log)
	default:
		return nil, errors.New(fmt.Sprintf("new block proposer failed, invalid scheduler type:%s", schedulerType))
	}
}
