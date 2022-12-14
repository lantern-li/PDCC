/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result block with rwset
package result

import (
	"chainmaker.org/chainmaker/logger/v2"
	"fmt"
	"time"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// BlockWithRWSetSubscribeResult block with rwset subscribe result
type BlockWithRWSetSubscribeResult struct {
	store protocol.BlockchainStore
	log   *logger.CMLogger
}

// GetResultByBlockInfo get result by height
func (b *BlockWithRWSetSubscribeResult) GetResultByBlockInfo(blockInfo *commonPb.BlockInfo, filter func(block *commonPb.Block) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	txs, count := filter(blockInfo.Block)
	fileterElapsed := time.Since(start)

	start = time.Now()
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         blockInfo.Block.Header,
			Dag:            blockInfo.Block.Dag,
			AdditionalData: blockInfo.Block.AdditionalData,
			Txs:            txs,
		},
		RwsetList: blockInfo.RwsetList,
	})
	marshalElapsed := time.Since(start)
	stat := &Stat{
		ResultTxCount:   count,
		TotalTxCount:    blockInfo.Block.Header.TxCount,
		GetBlockElapsed: 0,
		FilterElapsed:   fileterElapsed.Milliseconds(),
		MarshalElapsed:  marshalElapsed.Milliseconds(),
	}
	if err != nil {
		return nil, stat, fmt.Errorf("data marshal fail, at [blockInfo:%d], %s", blockInfo, err)
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}

// GetResultByHeight get result by height
func (b *BlockWithRWSetSubscribeResult) GetResultByHeight(height uint64, filter func(*commonPb.Block) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	blockWithRWSet, err := b.store.GetBlockWithRWSets(height)
	getBlockElapsed := time.Since(start)

	if err != nil {
		return nil, nil, fmt.Errorf("get block with rwset failed, at [height:%d], %s", height, err)
	}

	if blockWithRWSet == nil {
		return nil, nil, nil
	}
	start = time.Now()
	txs, count := filter(blockWithRWSet.Block)
	filterElapsed := time.Since(start)

	start = time.Now()
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         blockWithRWSet.Block.Header,
			Dag:            blockWithRWSet.Block.Dag,
			AdditionalData: blockWithRWSet.Block.AdditionalData,
			Txs:            txs,
		},
		RwsetList: blockWithRWSet.TxRWSets,
	})
	marshalElapsed := time.Since(start)
	stat := &Stat{
		ResultTxCount:   count,
		TotalTxCount:    blockWithRWSet.Block.Header.TxCount,
		GetBlockElapsed: getBlockElapsed.Milliseconds(),
		FilterElapsed:   filterElapsed.Milliseconds(),
		MarshalElapsed:  marshalElapsed.Milliseconds(),
	}
	if err != nil {
		return nil, stat, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}

// GetType get current type
func (b BlockWithRWSetSubscribeResult) GetType() Type {
	return RWSetResultType
}
