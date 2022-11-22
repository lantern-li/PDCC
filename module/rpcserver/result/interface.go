/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package result interface
package result

import (
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
	// GetResult get result by height
	GetResult(height uint64, fn func(*commonPb.Block) []*commonPb.Transaction) (*commonPb.SubscribeResult, error)
	// GetType get current type
	GetType() Type
}

// NewSubscribeResult new subscribe result
func NewSubscribeResult(resultType Type, store protocol.BlockchainStore) SubscribeResult {
	switch resultType {
	case BlockResultType:
		return &BlockSubscribeResult{store: store}
	case OnlyHeaderResultType:
		return &BlockHeaderSubscribeResult{store: store}
	case RWSetResultType:
		return &BlockWithRWSetSubscribeResult{store: store}
	default:
		return &BlockSubscribeResult{store: store}
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
