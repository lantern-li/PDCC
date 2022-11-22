/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result block with rwset
package result

import (
	"fmt"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// BlockWithRWSetSubscribeResult block with rwset subscribe result
type BlockWithRWSetSubscribeResult struct {
	store protocol.BlockchainStore
}

// GetResult get result by height
func (b *BlockWithRWSetSubscribeResult) GetResult(height uint64, filter func(*commonPb.Block) []*commonPb.Transaction) (
	*commonPb.SubscribeResult, error) {
	blockWithRWSet, err := b.store.GetBlockWithRWSets(height)
	if err != nil {
		return nil, fmt.Errorf("get block with rwset failed, at [height:%d], %s", height, err)
	}
	if blockWithRWSet == nil {
		return nil, nil
	}

	txs := filter(blockWithRWSet.Block)
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         blockWithRWSet.Block.Header,
			Dag:            blockWithRWSet.Block.Dag,
			AdditionalData: blockWithRWSet.Block.AdditionalData,
			Txs:            txs,
		},
		RwsetList: blockWithRWSet.TxRWSets,
	})
	if err != nil {
		return nil, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	return &commonPb.SubscribeResult{Data: data}, nil
}

// GetType get current type
func (b BlockWithRWSetSubscribeResult) GetType() Type {
	return RWSetResultType
}
