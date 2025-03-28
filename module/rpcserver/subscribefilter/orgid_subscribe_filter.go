/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// orgIdRule存在的问题
// 1. vm-native中initFilterRule中强制将StartHeight和EndHeight置为0，导致首次registerRule无效，且save的key是固定的/rule/0
// 2. 注册新规则时start_height无效果，因为是根据最新区块高度来设置下一个规则的开始区块。需要用更新来做
// 3. 如果将某个区块区间置为无效，如果这个区块之间有更新规则交易，则无法更新缓存

// Package subscribefilter subscriber filter
package subscribefilter

import (
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
	"path"
	"strconv"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	// 方法通配符
	// 因为orgId的规则与方法名无关，filterRules是通用的结构，为了适配这个结构，用通配符占用
	wildcardMethod = "*"
)

// OrgIdSubscriberFilter org id subscriber filter
type OrgIdSubscriberFilter struct {
	filterManager *SubscribeFilterManager
}

func (f *OrgIdSubscriberFilter) GetStore() protocol.BlockchainStore {
	return f.filterManager.store
}

func (f *OrgIdSubscriberFilter) GetType() txassign.RuleType {
	return txassign.RuleType_OrgId
}

// newOrgIdSubscriberFilter new org id subscriber filter
func newOrgIdSubscriberFilter(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger) (
	SubscribeFilter, error) {
	filterManager, err := newSubscribeFilterManager(tx, store, role, log)
	if err != nil {
		return nil, err
	}

	return &OrgIdSubscriberFilter{
		filterManager: filterManager,
	}, nil
}

// GetSubscribeFilterManagement get subscribe filter management
func (f *OrgIdSubscriberFilter) GetSubscribeFilterManagement() *SubscribeFilterManager {
	return f.filterManager
}

func (f OrgIdSubscriberFilter) GetSubscriber() *IdentityCode {
	return nil
}

// FilterTxs filter txs
func (f *OrgIdSubscriberFilter) FilterTxs(current *commonPb.Block, withTxId bool) (result []*commonPb.Transaction, count int) {
	// 非轻节点返回所有交易
	result = make([]*commonPb.Transaction, 0, current.Header.TxCount)

	filterRules, err := f.FilterRule(true)
	if err != nil {
		f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("[%v] get filter rule fail. txs: %v, error: %v", current.Header.BlockHeight, len(current.Txs), err))
		return
	}
	filterRule := filterRules[wildcardMethod]
	var rule *txassign.OrgIdRule
	for _, rule0 := range filterRule.OrgId {
		if checkRules(current.Header.BlockHeight, rule0.Rule, f.filterManager.Log, orgPrefix) {
			rule = rule0
			break
		}
	}
	if rule == nil {
		f.filterManager.Log.Debug("%s %s no rules available", ruleHelperPrefix, orgPrefix)
		return
	}
	f.filterManager.Log.Debugf("%s %s current rule [status:%v,start:%v,end:%v]", ruleHelperPrefix, orgPrefix,
		rule.Rule.Status, rule.Rule.StartHeight, rule.Rule.EndHeight)
	for _, tx := range current.Txs {
		if tx.Sender == nil {
			f.filterManager.Log.Debugf("%s %s [%v] rule value bounds out of range, rule: %v", ruleHelperPrefix, orgPrefix,
				tx.Payload.TxId)
			continue
		}
		// 更新rule缓存
		// warning: 如果latest rule的status是false，则不会走到更新缓存逻辑，已存在的订阅不会感知到rule更新
		if err := f.filterManager.updateOrgIdRuleCache(tx, current.Header.BlockHeight); err != nil {
			f.filterManager.Log.Errorf("updateOrgIdRuleCache error: %v", err.Error())
		}

		if tx.Sender.Signer.OrgId == f.filterManager.Tx.Sender.Signer.OrgId {
			result = append(result, tx)
			f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s [%v] rule match [status:%v,start:%v,end:%v,txs:%v,sub:%v,sender:%s]", ruleHelperPrefix, orgPrefix,
				current.Header.BlockHeight, rule.Rule.Status, rule.Rule.StartHeight, rule.Rule.EndHeight, len(result), f.filterManager.Tx.Sender.Signer.OrgId, tx.Sender.Signer.OrgId))
			continue
		}

		f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s [%v] not match [status:%v,start:%v,end:%v,txs:%v,sub:%v,sender:%s]", ruleHelperPrefix, orgPrefix,
			current.Header.BlockHeight, rule.Rule.Status, rule.Rule.StartHeight, rule.Rule.EndHeight, len(result), f.filterManager.Tx.Sender.Signer.OrgId, tx.Sender.Signer.OrgId))

	}
	f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s [%v] match over [status:%v,start:%v,end:%v,txs:%v]", ruleHelperPrefix, orgPrefix,
		current.Header.BlockHeight, rule.Rule.Status, rule.Rule.StartHeight, rule.Rule.EndHeight, len(result)))
	return
}

// getKey get key
func (f *OrgIdSubscriberFilter) getKey(_ string) string {
	format := strconv.FormatInt(int64(txassign.RuleType_OrgId), 10)
	return path.Join(RulePrefix, format)
}

// FilterRule get filter rule
func (f *OrgIdSubscriberFilter) FilterRule(cache bool) (map[string]*txassign.FilterRule, error) {
	key := f.getKey("")
	filterRule, err := f.filterManager.getFilterRule(key, cache)

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
		f.filterManager.ruleCache.Put(0, key, filterRule)
	}

	m := make(map[string]*txassign.FilterRule, 0)
	m[wildcardMethod] = filterRule
	return m, nil
}

// Validate parameters
func (f *OrgIdSubscriberFilter) Validate() error {
	return f.filterManager.validate()
}
