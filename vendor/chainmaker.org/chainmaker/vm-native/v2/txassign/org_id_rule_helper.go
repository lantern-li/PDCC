/*
 * Copyright (C) BABEC. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package txassign orgid
package txassign

import (
	"fmt"
	"path"
	"strconv"

	"chainmaker.org/chainmaker/common/v2/json"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

func newOrgIdHelper4bytes(bytes []byte, context protocol.TxSimContext) (RuleHelper, error) {
	var rule syscontract.OrgIdRule
	err := json.Unmarshal(bytes, &rule)
	if err != nil {
		return nil, fmt.Errorf("init halper unmarshal fail, error: %v", err)
	}
	return &orgIdRuleHelper{
		context: context,
		rule:    &txassign.OrgIdRule{Rule: &txassign.Rule{Status: rule.Status}},
	}, nil
}

func newOrgIdHelper4rule(context protocol.TxSimContext) (RuleHelper, error) {
	return &orgIdRuleHelper{context: context}, nil
}

type orgIdRuleHelper struct {
	context protocol.TxSimContext
	rule    *txassign.OrgIdRule
}

func (h *orgIdRuleHelper) rangeUpdate(fn func(i int, rule *txassign.Rule) bool) error {
	rule, err := h.getFilterRule()
	if err != nil {
		return err
	}
	for i, orgId := range rule.OrgId {
		if fn(i, orgId.Rule) {
			orgId.Rule.Status = h.rule.Rule.Status
			break
		}
	}
	err = h.saveFilterRule(rule)
	if err != nil {
		return err
	}
	return nil
}

func (h *orgIdRuleHelper) getLastRule(rule *txassign.FilterRule) ([]byte, error) {
	if len(rule.OrgId) == 0 {
		return nil, nil
	}
	orgIdRule := rule.OrgId[len(rule.OrgId)-1]
	bytes, err := json.Marshal(orgIdRule)
	if err != nil {
		return nil, fmt.Errorf("get last rule marshal fail, error: %v", err)
	}
	return bytes, nil
}

func (h *orgIdRuleHelper) saveFilterRule(filterRule *txassign.FilterRule) error {
	bytes, err := proto.Marshal(filterRule)
	if err != nil {
		return err
	}
	err = h.context.Put(contractName, h.getKey(), bytes)
	if err != nil {
		return err
	}
	return nil
}

func (h *orgIdRuleHelper) initFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule {
	var rules []*txassign.OrgIdRule
	rules = append(rules, h.rule)
	// 设置初始高度
	h.rule.Rule.StartHeight = 0
	h.rule.Rule.EndHeight = 0
	filterRule = &txassign.FilterRule{
		Id:    txassign.RuleType_OrgId,
		OrgId: rules,
	}
	return filterRule
}

func (h *orgIdRuleHelper) appendFilterRule(filterRule *txassign.FilterRule) *txassign.FilterRule {
	height := h.context.GetBlockHeight()
	// 设置上一规则为当前高度
	filterRule.OrgId[len(filterRule.OrgId)-1].Rule.EndHeight = height
	// 设置当前规则为当前高度+1
	h.rule.Rule.StartHeight = height + 1
	h.rule.Rule.EndHeight = 0
	filterRule.OrgId = append(filterRule.OrgId, h.rule)
	return filterRule
}

func (h *orgIdRuleHelper) getFilterRule() (*txassign.FilterRule, error) {
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

// getRuleKey get OrgId rule db keys
func (h *orgIdRuleHelper) getKey() []byte {
	format := strconv.FormatInt(int64(txassign.RuleType_OrgId), 10)
	return []byte(path.Join(r, format))
}
