/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result interface
package result

import (
	"chainmaker.org/chainmaker/logger/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

type Type uint32

const (
	// BlockResultType block result type
	BlockResultType Type = iota
	// OnlyHeaderResultType only header result type
	OnlyHeaderResultType
	// RWSetResultType rwset result type
	RWSetResultType
)

// ResultTypeNames key: Type, value: Type string
var ResultTypeNames = map[Type]string{
	BlockResultType:      "BlockResult",
	OnlyHeaderResultType: "HeaderResult",
	RWSetResultType:      "RWSetResult",
}

// SubscribeResult subscribe results
type SubscribeResult interface {
	// GetResultByBlockInfo get result by blockinfo
	GetResultByBlockInfo(blockInfo *commonPb.BlockInfo, fn func(*commonPb.Block) []*commonPb.Transaction) (*commonPb.SubscribeResult, error)
	// GetResultByHeight get result by height
	GetResultByHeight(height uint64, fn func(*commonPb.Block) []*commonPb.Transaction) (*commonPb.SubscribeResult, error)
	// GetType get current type
	GetType() Type
}

// NewSubscribeResult new subscribe result
func NewSubscribeResult(resultType Type, store protocol.BlockchainStore, log *logger.CMLogger) SubscribeResult {
	switch resultType {
	case BlockResultType:
		return &BlockSubscribeResult{store: store, log: log}
	case OnlyHeaderResultType:
		return &BlockHeaderSubscribeResult{store: store, log: log}
	case RWSetResultType:
		return &BlockWithRWSetSubscribeResult{store: store, log: log}
	default:
		return &BlockSubscribeResult{store: store, log: log}
	}
}

// GetResultType GetType get current type
func GetResultType(onlyHeader, withRWSet bool) Type {
	if onlyHeader {
		return OnlyHeaderResultType
	}
	if withRWSet {
		return RWSetResultType
	}
	return BlockResultType
}
