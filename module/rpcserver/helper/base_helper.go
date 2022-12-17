/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper base
package helper

import (
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// BaseHelper base helper
type BaseHelper struct {
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

// newBaseHelper new base helper
func newBaseHelper(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger) (
	*BaseHelper, error) {
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
	return &BaseHelper{
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

// NewHelper new helper
func NewHelper(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger, pool *ants.Pool) (Helper, error) {
	ruleType, err := getRuleType(tx.Payload.Parameters)
	if err != nil {
		return nil, err
	}
	switch ruleType {
	case txassign.RuleType_OrgId:
		return newOrgIdHelper(tx, store, role, log)
		//return nil, fmt.Errorf("rule type %v not support", ruleType)
	case txassign.RuleType_Alias:
		return newAliasHelper(tx, store, role, log, pool)
	default:
		return nil, fmt.Errorf("rule type %v not support", ruleType)
	}
}

// validate parameter
func (h BaseHelper) validate() error {
	if h.Start < -1 || h.End < -1 || (h.End != -1 && h.Start > h.End) {
		return errors.New("invalid Start block height or End block height")
	}
	if int64(h.LastBlockHeight) < h.Start {
		return fmt.Errorf("last block height:%d < payload Start block height:%d", int64(h.LastBlockHeight), h.Start)
	}
	return nil
}

// getFilterRule Get *txassign.FilterRule from cache or DB based on key
func (h *BaseHelper) getFilterRule(key string, cache bool) (*txassign.FilterRule, error) {
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
	if bytes == nil {
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
