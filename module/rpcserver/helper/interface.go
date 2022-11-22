/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper interface
package helper

import (
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
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
	// Verify block
	Verify(block *commonPb.Block) []*commonPb.Transaction
	// GetBaseHelper get *BaseHelper
	GetBaseHelper() *BaseHelper

	GetType() txassign.RuleType
}
