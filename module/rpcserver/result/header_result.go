/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result header
package result

import (
	"chainmaker.org/chainmaker/logger/v2"
	"fmt"

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
func (b BlockHeaderSubscribeResult) GetResultByBlockInfo(blockInfo *commonPb.BlockInfo, _ func(*commonPb.Block) []*commonPb.Transaction) (*commonPb.SubscribeResult, error) {
	data, err := proto.Marshal(blockInfo.Block.Header)
	if err != nil {
		return nil, fmt.Errorf("data marshal fail, at [blockInfo:%d], %s", blockInfo, err)
	}
	return &commonPb.SubscribeResult{Data: data}, nil
}

// GetResultByHeight get result by height
func (b BlockHeaderSubscribeResult) GetResultByHeight(height uint64, _ func(*commonPb.Block) []*commonPb.Transaction) (
	*commonPb.SubscribeResult, error) {
	header, err := b.store.GetBlockHeaderByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("get block failed, at [height:%d], %s", height, err)
	}
	if header == nil {
		return nil, nil
	}
	data, err := proto.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	return &commonPb.SubscribeResult{Data: data}, nil
}
