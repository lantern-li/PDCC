/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package proposer

import (
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func (bp *BlockProposerImpl) generateNewBlock(proposingHeight uint64, preHash []byte,
	txBatch []*commonpb.Transaction, batchIds []string, fetchBatches [][]*commonpb.Transaction) (
	*commonpb.Block, []int64, error) {

	return bp.blockBuilder.GenerateNewBlock(
		proposingHeight,
		preHash,
		txBatch,
		batchIds,
		fetchBatches)
}
func (bp *DeterministicBlockProposerImpl) generateNewBlock(proposingHeight uint64,
	preHash []byte, txBatch []*commonpb.Transaction,
	batchIds []string, fetchBatches [][]*commonpb.Transaction) (
	*commonpb.Block, error) {

	//// For SOLO consensus with deterministic scheduling, we need to execute transactions in propose phase
	//// because SOLO consensus implementation expects transactions to be executed
	//isSolo := bp.chainConf.ChainConfig().Consensus.Type == 0 // ConsensusType_SOLO
	//if isSolo {
	//	block, _, err := bp.blockBuilder.GenerateNewBlock(
	//		proposingHeight, preHash, txBatch, batchIds, fetchBatches)
	//	return block, err
	//}

	return bp.blockBuilder.GenerateNewPreBlock(
		proposingHeight, preHash, txBatch, batchIds, fetchBatches)
}
