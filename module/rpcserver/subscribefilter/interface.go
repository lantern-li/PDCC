/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package subscribefilter subscribe filter interface
package subscribefilter

import (
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	// RulePrefix rule key prefix
	RulePrefix = "/rule"

	ruleHelperPrefix = "[RuleHelper]"
	aliasPrefix      = "[alias]"
	orgPrefix        = "[org]"
)

// SubscribeFilter interface
type SubscribeFilter interface {
	// Validate parameters
	Validate() error
	// FilterRule get filter rule
	FilterRule(cache bool) (map[string]*txassign.FilterRule, error)
	// getKey get key
	getKey(method string) string
	// FilterTxs filter txs by block
	FilterTxs(block *commonPb.Block, withTxId bool) (result []*commonPb.Transaction, count int)
	// GetSubscribeFilterManagement get subscribe filter
	GetSubscribeFilterManagement() *SubscribeFilterManager

	GetSubscriber() *IdentityCode

	GetType() txassign.RuleType

	GetStore() protocol.BlockchainStore
}
