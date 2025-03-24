/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper interface
package helper

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

// Helper interface
type Helper interface {
	// Validate parameters
	Validate() error
	// FilterRule get filter rule
	FilterRule(cache bool) (map[string]*txassign.FilterRule, error)
	// getKey get key
	getKey(method string) string
	// FiltTxs filt txs by block
	FiltTxs(block *commonPb.Block, withTxId bool) (result []*commonPb.Transaction, count int)
	// GetBaseHelper get *BaseHelper
	GetBaseHelper() *BaseHelper

	GetSubscriber() *IdentityCode

	GetType() txassign.RuleType

	GetStore() protocol.BlockchainStore
}
