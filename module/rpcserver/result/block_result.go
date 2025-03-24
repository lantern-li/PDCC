/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result header
package result

import (
	"chainmaker.org/chainmaker/logger/v2"
	"fmt"
	"time"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// BlockSubscribeResult block subscribe result
type BlockSubscribeResult struct {
	store protocol.BlockchainStore
	log   *logger.CMLogger
}

// GetType get current type
func (b BlockSubscribeResult) GetType() Type {
	return BlockResultType
}

// GetResultByBlockInfo get result by BlockInfo
func (b BlockSubscribeResult) GetResultByBlockInfo(blockInfo *commonPb.BlockInfo, filter func(block *commonPb.Block) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	transactions, count := filter(blockInfo.Block)
	filterElapsed := time.Since(start)

	start = time.Now()
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         blockInfo.Block.Header,
			Dag:            blockInfo.Block.Dag,
			AdditionalData: blockInfo.Block.AdditionalData,
			Txs:            transactions,
		},
	})
	marshalElapsed := time.Since(start)

	stat := &Stat{
		ResultTxCount:  count,
		TotalTxCount:   blockInfo.Block.Header.TxCount,
		FilterElapsed:  filterElapsed.Milliseconds(),
		MarshalElapsed: marshalElapsed.Milliseconds(),
	}

	if err != nil {
		return nil, stat, fmt.Errorf("data marshal fail, at [height:%d], %s", blockInfo.Block.Header.BlockHeight, err)
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}

// GetResultByHeight get result by height
func (b BlockSubscribeResult) GetResultByHeight(height uint64, filter func(*commonPb.Block, bool) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	block, err := b.store.GetBlock(height)
	if err != nil {
		return nil, nil, fmt.Errorf("get block failed, at [height:%d], %s", height, err)
	}
	if block == nil {
		return nil, nil, nil
	}
	getBlockElapsed := time.Since(start)

	start = time.Now()
	transactions, count := filter(block, true)
	filterElapsed := time.Since(start)
	start = time.Now()
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         block.Header,
			Dag:            block.Dag,
			AdditionalData: block.AdditionalData,
			Txs:            transactions,
		},
	})
	marshalElapsed := time.Since(start)
	stat := &Stat{
		ResultTxCount:   count,
		TotalTxCount:    block.Header.TxCount,
		GetBlockElapsed: getBlockElapsed.Milliseconds(),
		FilterElapsed:   filterElapsed.Milliseconds(),
		MarshalElapsed:  marshalElapsed.Milliseconds(),
	}
	if err != nil {
		return nil, stat, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}
