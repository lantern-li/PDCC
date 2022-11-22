/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result header
package result

import (
	"fmt"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// BlockSubscribeResult block subscribe result
type BlockSubscribeResult struct {
	store protocol.BlockchainStore
}

// GetType get current type
func (b BlockSubscribeResult) GetType() Type {
	return BlockResultType
}

// GetResult get result by height
func (b BlockSubscribeResult) GetResult(height uint64, filter func(*commonPb.Block) []*commonPb.Transaction) (
	*commonPb.SubscribeResult, error) {
	block, err := b.store.GetBlock(height)
	if err != nil {
		return nil, fmt.Errorf("get block failed, at [height:%d], %s", height, err)
	}
	if block == nil {
		return nil, nil
	}
	transactions := filter(block)
	data, err := proto.Marshal(&commonPb.BlockInfo{
		Block: &commonPb.Block{
			Header:         block.Header,
			Dag:            block.Dag,
			AdditionalData: block.AdditionalData,
			Txs:            transactions,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("data marshal fail, at [height:%d], %s", height, err)
	}
	return &commonPb.SubscribeResult{Data: data}, nil
}
