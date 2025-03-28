/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package subscribefilter Package subscribe filter
package subscribefilter

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"path"
	"strconv"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

const nameRule = "rule"

// SubscribeFilterManager subscription filter manager
type SubscribeFilterManager struct {
	// Start height
	Start int64
	// End height
	End int64
	// LastBlockHeight last block height
	LastBlockHeight uint64
	// Log log
	Log protocol.Logger
	// subscribe Tx
	Tx *commonPb.Transaction
	// blockchain store
	store protocol.BlockchainStore
	// ruleCache rule cache key: /rule/0 or /rule/1/contractName/method value: *txassign.FilterRule
	ruleCache *ruleCache
	// user role
	role protocol.Role
}

// newSubscribeFilterManager new subscriber filter manager
func newSubscribeFilterManager(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger) (
	*SubscribeFilterManager, error) {
	start, err := GetParameterInt64(tx.Payload.Parameters, syscontract.SubscribeBlock_START_BLOCK.String())
	if err != nil {
		return nil, err
	}
	end, err := GetParameterInt64(tx.Payload.Parameters, syscontract.SubscribeBlock_END_BLOCK.String())
	if err != nil {
		return nil, err
	}
	block, err := store.GetLastBlock()
	if err != nil {
		return nil, err
	}
	return &SubscribeFilterManager{
		Start:           start,
		End:             end,
		LastBlockHeight: block.Header.BlockHeight,
		Log:             log,
		Tx:              tx,
		store:           store,
		ruleCache:       Factory().cache,
		role:            role,
	}, nil
}

// InitSubscribeFilter init subscribe filter
func InitSubscribeFilter(tx *commonPb.Transaction,
	store protocol.BlockchainStore,
	role protocol.Role,
	log protocol.Logger,
	pool *ants.Pool) (SubscribeFilter, error) {
	// new helper
	filter, err := NewSubscribeFilter(tx, store, role, log, pool)
	if err != nil {
		return nil, err
	}
	err = filter.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate parameter fail, error: %v", err)
	}
	// get filter rule from db
	rule, err := filter.FilterRule(false)
	if err != nil {
		return nil, err
	}
	// 需求如此，必须注册清分规则后，才允许清分
	if rule == nil {
		parameters, _ := json.Marshal(tx.Payload.Parameters)
		return nil, fmt.Errorf("%v rule, filter rule not found, parameters: %v", filter.GetType().String(), parameters)
	}
	return filter, nil
}

// NewSubscribeFilter new susbcribe filter
func NewSubscribeFilter(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger, pool *ants.Pool) (SubscribeFilter, error) {
	ruleType, err := getRuleType(tx.Payload.Parameters)
	if err != nil {
		return nil, err
	}
	switch ruleType {
	case txassign.RuleType_OrgId:
		return newOrgIdSubscriberFilter(tx, store, role, log)
		//return nil, fmt.Errorf("rule type %v not support", ruleType)
	case txassign.RuleType_Alias:
		return newAliasSubscribeFilter(tx, store, role, log, pool)
	default:
		return nil, fmt.Errorf("rule type %v not support", ruleType)
	}
}

// validate parameter
func (h SubscribeFilterManager) validate() error {
	if h.Start < -1 || h.End < -1 || (h.End != -1 && h.Start > h.End) {
		return errors.New("invalid Start block height or End block height")
	}
	if int64(h.LastBlockHeight) < h.Start {
		return fmt.Errorf("last block height:%d < payload Start block height:%d", int64(h.LastBlockHeight), h.Start)
	}
	return nil
}

// getFilterRule Get *txassign.FilterRule from cache or DB based on key
func (h *SubscribeFilterManager) getFilterRule(key string, cache bool) (*txassign.FilterRule, error) {
	var (
		filterRule *txassign.FilterRule
		height     uint64
	)
	h.ruleCache.l.Lock()
	defer h.ruleCache.l.Unlock()
	if cache {
		filterRule, height = h.ruleCache.Get(key)
		if filterRule != nil {
			return filterRule, nil
		}
	}
	// Does not exist in cache, query DB
	bytes, err := h.store.ReadObject(syscontract.SystemContract_TX_ASSIGN.String(), []byte(key))
	if err != nil {
		return nil, err
	}
	// There is no return nil in DB
	if bytes == nil || len(bytes) == 0 {
		return nil, nil
	}
	if h.LastBlockHeight > height {
		// If present in DB, deserialize to object
		filterRule = &txassign.FilterRule{}
		err = proto.Unmarshal(bytes, filterRule)
		if err != nil {
			return nil, err
		}
		// 更新缓存
		h.ruleCache.Put(h.LastBlockHeight, key, filterRule)
	}
	return filterRule, nil
}

// updateAliasRuleCache update alias rule cache
func (h *SubscribeFilterManager) updateAliasRuleCache(tx *commonPb.Transaction, height uint64) error {
	contract := tx.Payload.ContractName
	method := tx.Payload.Method
	if tx.Result.ContractResult.Code == 0 &&
		contract == syscontract.SystemContract_TX_ASSIGN.String() &&
		(method == syscontract.TxAssignFunction_RegisterRule.String() ||
			method == syscontract.TxAssignFunction_UpdateRuleByHeight.String()) {

		var txAssignContractName, txAssignMethodName string
		for _, kv := range tx.Payload.Parameters {
			if kv.Key == nameRule {
				rule := &syscontract.AliasRule{}
				err := json.Unmarshal(kv.Value, rule)
				if err != nil {
					return fmt.Errorf("unmarshall %v error: %v", nameRule, err.Error())
				}
				txAssignContractName = rule.ContractName
				txAssignMethodName = rule.Method
			}
		}
		h.ruleCache.l.Lock()
		defer h.ruleCache.l.Unlock()
		format := strconv.FormatInt(int64(txassign.RuleType_Alias), 10)
		key := path.Join(RulePrefix, format, txAssignContractName, txAssignMethodName)
		bytes, err := h.store.ReadObject(syscontract.SystemContract_TX_ASSIGN.String(), []byte(key))
		if err != nil {
			return fmt.Errorf("read rule [%v] error: %v", key, err.Error())
		}
		if bytes == nil || len(bytes) == 0 {
			return fmt.Errorf("read rule [%v] empty", key)
		}
		filterRule := &txassign.FilterRule{}
		err = proto.Unmarshal(bytes, filterRule)
		if err != nil {
			return fmt.Errorf("unmarshall rule error: %v", err.Error())
		}
		// 更新缓存
		h.ruleCache.Put(height, key, filterRule)
	}
	return nil
}

// updateOrgIdRuleCache update orgId rule cache
func (h *SubscribeFilterManager) updateOrgIdRuleCache(tx *commonPb.Transaction, height uint64) error {
	contract := tx.Payload.ContractName
	method := tx.Payload.Method
	if tx.Result.ContractResult.Code == 0 &&
		contract == syscontract.SystemContract_TX_ASSIGN.String() &&
		(method == syscontract.TxAssignFunction_RegisterRule.String() ||
			method == syscontract.TxAssignFunction_UpdateRuleByHeight.String()) {

		rule := &txassign.Rule{}
		for _, kv := range tx.Payload.Parameters {
			if kv.Key == nameRule {
				err := json.Unmarshal(kv.Value, rule)
				if err != nil {
					return fmt.Errorf("unmarshall %v error: %v", nameRule, err.Error())
				}
			}
		}
		h.ruleCache.l.Lock()
		defer h.ruleCache.l.Unlock()
		format := strconv.FormatInt(int64(txassign.RuleType_OrgId), 10)
		key := path.Join(RulePrefix, format)

		bytes, err := h.store.ReadObject(syscontract.SystemContract_TX_ASSIGN.String(), []byte(key))
		if err != nil {
			return fmt.Errorf("read rule [%v] error: %v", key, err.Error())
		}
		if bytes == nil || len(bytes) == 0 {
			return fmt.Errorf("read rule [%v] empty", key)
		}

		filterRule := &txassign.FilterRule{}
		err = proto.Unmarshal(bytes, filterRule)
		if err != nil {
			return fmt.Errorf("unmarshall rule error: %v", err.Error())
		}
		// 更新缓存
		h.ruleCache.Put(height, key, filterRule)
	}
	return nil
}
