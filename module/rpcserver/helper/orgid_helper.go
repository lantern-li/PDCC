/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper orgid helper
package helper

import (
	"path"
	"strconv"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	fixMethod = "*"
)

// OrgIdHelper org id helper
type OrgIdHelper struct {
	helper *BaseHelper
}

func (h *OrgIdHelper) GetType() txassign.RuleType {
	return txassign.RuleType_OrgId
}

// newOrgIdHelper new org id helper
func newOrgIdHelper(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger) (
	Helper, error) {
	helper, err := newBaseHelper(tx, store, role, log)
	if err != nil {
		return nil, err
	}

	return &OrgIdHelper{
		helper: helper,
	}, nil
}

// GetBaseHelper get base helper
func (h *OrgIdHelper) GetBaseHelper() *BaseHelper {
	return h.helper
}

// Verify block
func (h *OrgIdHelper) Verify(current *commonPb.Block) (result []*commonPb.Transaction) {
	// 非轻节点返回所有交易
	if h.helper.role != protocol.RoleLight {
		h.helper.Log.Errorf("%s %s non-light nodes do not judge rules", ruleHelperPrefix, orgPrefix)
		return current.Txs
	}

	result = []*commonPb.Transaction{}

	filterRules, err := h.FilterRule(true)
	if err != nil {
		return
	}
	filterRule := filterRules[fixMethod]
	var rule *txassign.OrgIdRule
	for _, rule0 := range filterRule.OrgId {
		if checkRules(current.Header.BlockHeight, rule0.Rule, h.helper.Log, orgPrefix) {
			rule = rule0
			break
		}
	}
	if rule == nil {
		h.helper.Log.Debug("%s %s no rules available", ruleHelperPrefix, orgPrefix)
		return
	}
	h.helper.Log.Debugf("%s %s current rule [status:%v,start:%v,end:%v]", ruleHelperPrefix, orgPrefix,
		rule.Rule.Status, rule.Rule.StartHeight, rule.Rule.EndHeight)
	for _, tx := range current.Txs {
		if tx.Sender == nil {
			h.helper.Log.Debugf("%s %s [%v] rule value bounds out of range, rule: %v", ruleHelperPrefix, orgPrefix,
				tx.Payload.TxId)
			continue
		}
		if tx.Sender.Signer.OrgId == h.helper.Tx.Sender.Signer.OrgId {
			result = append(result, tx)
		}
	}
	return
}

// getKey get key
func (h *OrgIdHelper) getKey(_ string) string {
	format := strconv.FormatInt(int64(txassign.RuleType_OrgId), 10)
	return path.Join(RulePrefix, format)
}

// FilterRule get filter rule
func (h *OrgIdHelper) FilterRule(cache bool) (map[string]*txassign.FilterRule, error) {
	key := h.getKey("")
	filterRule, err := h.helper.getFilterRule(key, cache)
	if err != nil {
		return nil, err
	}
	if filterRule == nil {
		// Default orgid rule
		filterRule = &txassign.FilterRule{
			Id: txassign.RuleType_OrgId,
			OrgId: []*txassign.OrgIdRule{{Rule: &txassign.Rule{
				Status:      txassign.RuleStatus_Enabled,
				StartHeight: 0,
				EndHeight:   0,
			}}},
		}
		h.helper.ruleCache.Put(0, key, filterRule)
	}

	m := make(map[string]*txassign.FilterRule, 0)
	m[fixMethod] = filterRule
	return m, nil
}

// Validate parameters
func (h *OrgIdHelper) Validate() error {
	return h.helper.validate()
}
