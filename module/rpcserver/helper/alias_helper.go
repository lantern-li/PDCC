/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper alias
package helper

import (
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"path"
	"strconv"
	"strings"
	"sync"

	"chainmaker.org/chainmaker/common/v2/json"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
)

const sep = ","

// AliasHelper alias helper
type AliasHelper struct {
	// base helper
	helper *BaseHelper
	// contractName contract name
	contractName string
	// contract methods
	methods []string
	// subscriberId 订阅者（别名ID）
	subscriberId *IdentityCode

	pool *ants.Pool
}

func (h *AliasHelper) GetStore() protocol.BlockchainStore {
	return h.helper.store
}

func (h *AliasHelper) GetType() txassign.RuleType {
	return txassign.RuleType_Alias
}

// newAliasHelper
func newAliasHelper(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger, pool *ants.Pool) (*AliasHelper, error) {
	contractName, method, err := getParameters(tx.Payload.Parameters)
	if err != nil {
		return nil, err
	}
	helper, err := newBaseHelper(tx, store, role, log)
	if err != nil {
		return nil, err
	}
	subscriberId, err := GetIdentityCode(string(tx.Sender.Signer.MemberInfo))
	if err != nil {
		return nil, fmt.Errorf("%v, subscriberId: %v", err, string(tx.Sender.Signer.MemberInfo))
	}

	return &AliasHelper{
		contractName: contractName,
		methods:      strings.Split(method, sep),
		helper:       helper,
		subscriberId: subscriberId,
		pool:         pool,
	}, nil
}

// GetBaseHelper get *BaseHelper
func (h *AliasHelper) GetBaseHelper() *BaseHelper {
	return h.helper
}

func (h *AliasHelper) GetSubscriber() *IdentityCode {
	return h.subscriberId
}

// FiltTxs filt txs by block
func (h *AliasHelper) FiltTxs(current *commonPb.Block, withTxId bool) (result []*commonPb.Transaction, count int) {
	txs := current.Txs
	height := current.Header.BlockHeight
	filterRules, err := h.FilterRule(true)
	if err != nil {
		h.helper.Log.Errorf("%s %s get filter rules fail, error: %v", ruleHelperPrefix, aliasPrefix, err)
		return
	}
	if len(filterRules) != len(h.methods) {
		h.helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s no rules available, methods: %v, filterRules: %v", ruleHelperPrefix, aliasPrefix, h.methods, filterRules))
		return
	}

	if len(filterRules) == 0 {
		return
	}
	var (
		rules = make(map[string]*txassign.AliasRule)
	)
	for method, filterRule := range filterRules {
		if filterRule == nil {
			continue
		}

		for _, rule0 := range filterRule.Alias {
			if checkRules(height, rule0.Rule, h.helper.Log, aliasPrefix) {
				rules[method] = rule0
				break
			}
		}
	}
	if len(rules) != len(h.methods) {
		fmt.Printf(">>> len(rules) != len(h.methods): %v,%v \n", len(rules), len(h.methods))
		h.helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s no rules available, ", ruleHelperPrefix, aliasPrefix))
		return
	}

	var (
		wg            = &sync.WaitGroup{}
		matchTxsIndex = make([]bool, current.Header.TxCount)
	)
	batchIndexes := DispatchTxVerifyTask(int(current.Header.TxCount))
	total := len(batchIndexes)
	for _, method := range h.methods {
		for i, index := range batchIndexes {
			wg.Add(1)
			batch := i
			startIndex := index[0]
			endIndex := index[1]
			method1 := method
			h.helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("[%v] run batch, total:%v,batch:%v,startIndex:%v,endIndex:%v", height, total, batch, startIndex, endIndex))
			err = h.pool.Submit(func() {
				verifyTxs(wg, h.helper, height, rules[method1], txs, method1, h.contractName, h.subscriberId, matchTxsIndex, total, batch, startIndex, endIndex)
			})
			if err != nil {
				h.helper.Log.Errorf("subscribe pool submit fail. error: %v", err)
				return
			}
		}
	}
	var transactions = make([]*commonPb.Transaction, 0, len(txs))
	wg.Wait()
	var resultTxCount int
	for i, match := range matchTxsIndex {
		if match {
			resultTxCount++
			transactions = append(transactions, txs[i])
		} else {
			if withTxId {
				// not match tx
				transactions = append(transactions, &commonPb.Transaction{
					Payload: &commonPb.Payload{
						TxId: txs[i].Payload.TxId,
					},
					Result: &commonPb.Result{RwSetHash: txs[i].Result.RwSetHash},
				})
			}
		}
	}
	return transactions, resultTxCount
}

func verifyTxs(wg *sync.WaitGroup, helper *BaseHelper, height uint64, rule *txassign.AliasRule, txs []*commonPb.Transaction, method, contractName string, subscriberId *IdentityCode, matchTxsIndex []bool, total, batch, startIndex, endIndex int) {
	helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("run batch, total:%v,batch:%v,startIndex:%v,endIndex:%v", total, batch, startIndex, endIndex))
MatchSuccessfulToVerifyTheNextTransaction:
	for index := startIndex; index < endIndex; index++ {
		tx := txs[index]

		// 更新rule缓存
		helper.updateRuleCache(tx, height)

		if contractName != tx.Payload.ContractName || method != tx.Payload.Method {
			helper.Log.DebugDynamic(func() string {
				bytes, _ := json.Marshal(rule)
				return fmt.Sprintf("%s %s [%s] [%v] [%v] contract name or methods do not match, contract_name(tx:%v,rule:%v), "+
					"methods(tx:%v,rule:%v), rule: %v", ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, tx.Payload.ContractName, contractName, tx.Payload.Method, method, string(bytes))
			})
			continue
		}

		for _, name := range strings.Split(rule.Name, sep) {
			// Get the aliasValueString in the current transaction parameter based on the rule field name
			aliasValueString, err := getParameterString(tx.Payload.Parameters, name) // name = “param1，param2”
			if err != nil {
				helper.Log.DebugDynamic(func() string {
					ruleJson, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] [%v] get string parameter by rule name fail, rule: %v, error: %v", ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, string(ruleJson), err)
				})
				continue
			}
			// 判断总体
			if rule.Index+rule.Offset > uint32(len(aliasValueString)) {
				helper.Log.DebugDynamic(func() string {
					ruleJson, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] [%v] all rule value bounds out of range, value: %v, rule: %v", ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, aliasValueString, string(ruleJson))
				})
				continue
			}
			aliasValues := strings.Split(aliasValueString, sep)
			for i, aliasValue := range aliasValues {

				//if id.IdentityMatchInstance.Match(subscriberId, aliasValue) {

				participantId, err := GetIdentityCode(aliasValue)
				if err != nil {
					continue MatchSuccessfulToVerifyTheNextTransaction
				}
				if subscriberId.Match(participantId) {
					helper.Log.DebugDynamic(func() string {
						ruleJson, _ := json.Marshal(rule)
						return fmt.Sprintf("%s %s [%s] [%v] [%v] rule value match, i: %v, aliasValue: %v, "+
							"subscriberId: %v, rule: %v, ", ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, i, aliasValue, subscriberId, string(ruleJson))
					})
					matchTxsIndex[index] = true
					continue MatchSuccessfulToVerifyTheNextTransaction
				} else {
					helper.Log.DebugDynamic(func() string {
						ruleJson, _ := json.Marshal(rule)
						return fmt.Sprintf("%s %s [%s] [%v] [%v] rule value don't match, i: %v, aliasValue: %v, "+
							"subscriberId: %v, rule: %v, ", ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, i, aliasValue, subscriberId, string(ruleJson))
					})
					continue
				}
			}
		}
	}
	wg.Done()
}

// getKey get key
func (h *AliasHelper) getKey(method string) string {
	format := strconv.FormatInt(int64(txassign.RuleType_Alias), 10)
	return path.Join(RulePrefix, format, h.contractName, method)
}

// FilterRule get filter rule
func (h *AliasHelper) FilterRule(cache bool) (map[string]*txassign.FilterRule, error) {
	filterRules := make(map[string]*txassign.FilterRule, len(h.methods))
	for i, method := range h.methods {
		key := h.getKey(method)
		h.helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("get filter rule with key, %v", key))
		filterRule, err := h.helper.getFilterRule(key, cache)
		if err != nil {
			h.helper.Log.Errorf("filter rule not found. key: %v, i: %v", key, i)
			continue
		}
		filterRules[method] = filterRule
	}
	return filterRules, nil
}

// Validate parameters
func (h *AliasHelper) Validate() error {
	err := h.helper.validate()
	if err != nil {
		return err
	}
	if len(h.contractName) == 0 {
		return errors.New("contractName cannot empty")
	}
	if len(h.methods) == 0 {
		return errors.New("method cannot empty")
	}
	return nil
}

// getParameters get contractName, methods by parameters
func getParameters(parameters []*commonPb.KeyValuePair) (contractName string, method string, err error) {
	contractName, err = getParameterString(parameters, syscontract.SubscribeBlock_CONTRACT_NAME.String())
	if err != nil {
		return
	}
	method, err = getParameterString(parameters, syscontract.SubscribeBlock_METHOD.String())
	if err != nil {
		return
	}
	return
}
