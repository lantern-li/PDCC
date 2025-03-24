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

// BlockHeaderSubscribeResult block header subscribe result
type BlockHeaderSubscribeResult struct {
	store protocol.BlockchainStore
	log   *logger.CMLogger
}

// GetType get current type
func (b BlockHeaderSubscribeResult) GetType() Type {
	return OnlyHeaderResultType
}

// GetResultByBlockInfo get result by BlockInfo
func (b BlockHeaderSubscribeResult) GetResultByBlockInfo(blockInfo *commonPb.BlockInfo, _ func(block *commonPb.Block) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	data, err := proto.Marshal(blockInfo.Block.Header)
	if err != nil {
		return nil, nil, fmt.Errorf("data marshal fail, at [blockInfo:%d], %s", blockInfo, err)
	}
	marshalElapsed := time.Since(start)
	stat := &Stat{
		ResultTxCount:  0,
		TotalTxCount:   blockInfo.Block.Header.TxCount,
		FilterElapsed:  0,
		MarshalElapsed: marshalElapsed.Milliseconds(),
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}

// GetResultByHeight get result by height
func (b BlockHeaderSubscribeResult) GetResultByHeight(height uint64, _ func(*commonPb.Block, bool) (result []*commonPb.Transaction, count int)) (*commonPb.SubscribeResult, *Stat, error) {
	start := time.Now()
	header, err := b.store.GetBlockHeaderByHeight(height)
	getBlockElapsed := time.Since(start)

	if err != nil {
		return nil, nil, fmt.Errorf("get block failed, at [height:%d], %s", height, err)
	}
	if header == nil {
		return nil, nil, nil
	}
	start = time.Now()
	data, err := proto.Marshal(header)
	if err != nil {
		return nil, nil, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	marshalElapsed := time.Since(start)

	stat := &Stat{
		ResultTxCount:   0,
		TotalTxCount:    0,
		GetBlockElapsed: getBlockElapsed.Milliseconds(),
		FilterElapsed:   0,
		MarshalElapsed:  marshalElapsed.Milliseconds(),
	}
	return &commonPb.SubscribeResult{Data: data}, stat, nil
}
