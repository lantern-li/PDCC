/*
 * Copyright (C) BABEC. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package txassign alias
package txassign

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// validateAlias validate alias
func validateAlias(bytes []byte) (rule *syscontract.AliasRule, err error) {
	rule = &syscontract.AliasRule{}
	err = json.Unmarshal(bytes, rule)
	if err != nil {
		return nil, fmt.Errorf("init halper unmarshal fail, error: %v", err)
	}
	if rule.Index < 0 {
		return nil, errors.New("\"Index\" cannot be less than 0")
	}
	if rule.Offset < 0 {
		return nil, errors.New("\"Offset\" cannot be less than 0")
	}
	nameLength := uint32(len(rule.Name))
	if nameLength <= 0 {
		return nil, errors.New("\"Name\" length cannot be empty")
	}
	// check status
	_, ok := txassign.RuleStatus_name[int32(rule.Status)]
	if !ok {
		return nil, errors.New("\"rule.Status\" is invalid")
	}
	// check contract name
	if len(rule.ContractName) <= 0 {
		return nil, errors.New("\"ContractName\" length cannot be empty")
	}
	// check method
	if len(rule.Method) <= 0 {
		return nil, errors.New("\"Method\" length cannot be empty")
	}
	return
}

// newAliasHelper4bytes new newAliasHelper4rule for bytes
func newAliasHelper4bytes(bytes []byte, context protocol.TxSimContext) (RuleHelper, error) {
	// validate alias
	update, err := validateAlias(bytes)
	if err != nil {
		return nil, err
	}
	rule := &txassign.AliasRule{
		Rule: &txassign.Rule{
			Status:      update.Status,
			StartHeight: 0,
			EndHeight:   0,
		},
		Index:  update.Index,
		Offset: update.Offset,
		Name:   update.Name,
	}

	return &aliasRuleHelper{
		context:      context,
		rule:         rule,
		contractName: update.ContractName,
		method:       update.Method,
	}, nil
}

// newAliasHelper4rule new newAliasHelper4rule for rule
func newAliasHelper4rule(context protocol.TxSimContext, contractName string, method string) (RuleHelper, error) {
	return &aliasRuleHelper{
		context:      context,
		contractName: contractName,
		method:       method,
	}, nil
}

// aliasRuleHelper alias rule helper
type aliasRuleHelper struct {
	// context contract context
	context protocol.TxSimContext
	// rule struct
	rule         *txassign.AliasRule
	contractName string
	method       string
}

func (h *aliasRuleHelper) rangeUpdate(fn func(i int, rule *txassign.Rule) bool) error {
	filterRule, err := h.getFilterRule()
	if err != nil {
		return err
	}
	if filterRule == nil {
		return errors.New("rules don't exist")
	}
	for i, alias := range filterRule.Alias {
		if fn(i, alias.Rule) {
			alias.Rule.Status = h.rule.Rule.Status
			alias.Offset = h.rule.Offset
			alias.Index = h.rule.Index
			alias.Name = h.rule.Name
			break
		}
	}
	err = h.saveFilterRule(filterRule)
	if err != nil {
		return err
	}
	return nil
}

func (h *aliasRuleHelper) getLastRule(rule *txassign.FilterRule) ([]byte, error) {
	if len(rule.Alias) == 0 {
		return nil, nil
	}
	aliasRule := rule.Alias[len(rule.Alias)-1]
	bytes, err := json.Marshal(aliasRule)
	if err != nil {
		return nil, fmt.Errorf("get last rule marshal fail, error: %v", err)
	}
	return bytes, nil
}

func (h *aliasRuleHelper) saveFilterRule(filterRule *txassign.FilterRule) error {
	bytes, err := proto.Marshal(filterRule)
	if err != nil {
		return fmt.Errorf("save filter rule marshal fail, error: %v", err)
	}
	err = h.context.Put(contractName, h.getKey(), bytes)
	if err != nil {
		return fmt.Errorf("save filter rule fail, error: %v", err)
	}
	return nil
}

func (h *aliasRuleHelper) initFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule {
	var (
		rules []*txassign.AliasRule
		rule  = h.rule
	)

	// 设置初始高度
	rule.Rule.StartHeight = 0
	rule.Rule.EndHeight = 0
	rules = append(rules, h.rule)
	filterRule = &txassign.FilterRule{
		Id:    txassign.RuleType_Alias,
		Alias: rules,
	}
	return filterRule
}

func (h *aliasRuleHelper) appendFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule {
	height := h.context.GetBlockHeight()
	// 设置上一规则为当前高度
	filterRule.Alias[len(filterRule.Alias)-1].Rule.EndHeight = height
	// 设置当前规则为当前高度+1
	h.rule.Rule.StartHeight = height + 1
	h.rule.Rule.EndHeight = 0
	filterRule.Alias = append(filterRule.Alias, h.rule)
	return filterRule
}

func (h *aliasRuleHelper) getFilterRule() (*txassign.FilterRule, error) {
	bytes, err := h.context.Get(contractName, h.getKey())
	if err != nil {
		return nil, fmt.Errorf("get filter rule from db fail, error: %v", err)
	}
	if bytes == nil {
		return nil, nil
	}
	var filterRule txassign.FilterRule
	err = proto.Unmarshal(bytes, &filterRule)
	if err != nil {
		return nil, fmt.Errorf("filter rule unmarshal fail, error: %v", err)
	}
	return &filterRule, nil
}

// getRuleKey get alias rule db keys
func (h *aliasRuleHelper) getKey() []byte {
	format := strconv.FormatInt(int64(txassign.RuleType_Alias), 10)
	return []byte(path.Join(r, format, h.contractName, h.method))
}
