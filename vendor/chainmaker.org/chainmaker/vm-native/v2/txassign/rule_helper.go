/*
 * Copyright (C) BABEC. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package txassign utils
package txassign

import (
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"errors"
)

const r = "/rule"

func newHelper4byte(typ txassign.RuleType, rule []byte, context protocol.TxSimContext) (RuleHelper, error) {
	switch typ {
	case txassign.RuleType_OrgId:
		return newOrgIdHelper4bytes(rule, context)
	case txassign.RuleType_Alias:
		return newAliasHelper4bytes(rule, context)
	}
	return nil, errors.New("rule type not support")
}

func newHelper4rule(typ txassign.RuleType, context protocol.TxSimContext, contractName string, method string) (
	RuleHelper, error) {
	switch typ {
	case txassign.RuleType_OrgId:
		return newOrgIdHelper4rule(context)
	case txassign.RuleType_Alias:
		return newAliasHelper4rule(context, contractName, method)
	}
	return nil, errors.New("rule type not support")
}

type RuleHelper interface {
	getKey() []byte
	getFilterRule() (*txassign.FilterRule, error)
	initFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule
	appendFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule
	saveFilterRule(filterRule *txassign.FilterRule) error
	getLastRule(rule *txassign.FilterRule) ([]byte, error)
	rangeUpdate(fn func(i int, rule *txassign.Rule) bool) error
}
