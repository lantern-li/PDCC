/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper alias
package helper

import (
	"chainmaker.org/chainmaker-go/module/rpcserver/id"
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
	"errors"
	"fmt"
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
	subscriberId id.SubscriberId
}

func (h *AliasHelper) GetType() txassign.RuleType {
	return txassign.RuleType_Alias
}

// newAliasHelper
func newAliasHelper(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger) (
	*AliasHelper, error) {
	contractName, method, err := getParameters(tx.Payload.Parameters)
	if err != nil {
		return nil, err
	}
	helper, err := newBaseHelper(tx, store, role, log)
	if err != nil {
		return nil, err
	}
	subscriberId, err := id.NewSubscriberId(string(tx.Sender.Signer.MemberInfo))
	if err != nil {
		return nil, fmt.Errorf("%v, subscriberId: %v", err, string(tx.Sender.Signer.MemberInfo))
	}
	return &AliasHelper{
		contractName: contractName,
		methods:      strings.Split(method, sep),
		helper:       helper,
		subscriberId: subscriberId,
	}, nil
}

// GetBaseHelper get *BaseHelper
func (h *AliasHelper) GetBaseHelper() *BaseHelper {
	return h.helper
}

func (h AliasHelper) GetSubscriber() id.SubscriberId {
	return h.subscriberId
}

// Verify block
func (h *AliasHelper) Verify(current *commonPb.Block) (result []*commonPb.Transaction, count int) {
	result = []*commonPb.Transaction{}
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
			if checkRules(current.Header.BlockHeight, rule0.Rule, h.helper.Log, aliasPrefix) {
				rules[method] = rule0
				break
			}
		}
	}
	if len(rules) != len(h.methods) {
		h.helper.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s no rules available, ", ruleHelperPrefix, aliasPrefix))
		return
	}
	var (
		wg      = &sync.WaitGroup{}
		resultC = make(chan *commonPb.Transaction, current.Header.TxCount)
	)

	for _, method := range h.methods {
		wg.Add(1)
		go verifyTxs(wg, h.helper.Log, rules[method], current.Txs, method, h.contractName, h.subscriberId, resultC)
	}
	wg.Wait()
	close(resultC)
	var resultTxCount int
	// merge transactions
	for transaction := range resultC {
		if transaction.Payload.ContractName != "" {
			resultTxCount++
		}
		result = append(result, transaction)
	}
	return result, resultTxCount
}

func verifyTxs(wg *sync.WaitGroup, log protocol.Logger, rule *txassign.AliasRule, txs []*commonPb.Transaction, method, contractName string, subscriberId id.SubscriberId, result chan *commonPb.Transaction) {
MatchSuccessfulToVerifyTheNextTransaction:
	for _, tx := range txs {
		if contractName != tx.Payload.ContractName || method != tx.Payload.Method {
			log.DebugDynamic(func() string {
				bytes, _ := json.Marshal(rule)
				return fmt.Sprintf("%s %s [%s] [%v] contract name or methods do not match, contract_name(tx:%v,rule:%v), "+
					"methods(tx:%v,rule:%v), rule: %v", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, tx.Payload.ContractName, contractName, tx.Payload.Method, method, string(bytes))
			})
			continue
		}

		for _, name := range strings.Split(rule.Name, sep) {
			// Get the aliasValueString in the current transaction parameter based on the rule field name
			aliasValueString, err := getParameterString(tx.Payload.Parameters, name)
			if err != nil {
				log.DebugDynamic(func() string {
					ruleJson, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] get string parameter by rule name fail, rule: %v, error: %v", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, string(ruleJson), err)
				})
				continue
			}
			// 判断总体
			if rule.Index+rule.Offset > uint32(len(aliasValueString)) {
				log.DebugDynamic(func() string {
					ruleJson, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] all rule value bounds out of range, value: %v, rule: %v", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, aliasValueString, string(ruleJson))
				})
				continue
			}
			aliasValues := strings.Split(aliasValueString, sep)
			for i, aliasValue := range aliasValues {
				participantId, err := id.NewParticipantId(aliasValue)
				if err != nil {
					log.Warnf("%s %s [%s] [%v] new participant id error, participantId: %v, error: %v", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, aliasValue, err)
					continue
				}
				if id.IdentityMatchInstance.Match(subscriberId, participantId) {
					log.DebugDynamic(func() string {
						ruleJson, _ := json.Marshal(rule)
						return fmt.Sprintf("%s %s [%s] [%v] rule value match, i: %v, aliasValue: %v, "+
							"subscriberId: %v, memberInfo: %v, rule: %v, ", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, i, aliasValue, aliasValue[rule.Index:rule.Index+rule.Offset], subscriberId, string(ruleJson))
					})
					result <- tx
					continue MatchSuccessfulToVerifyTheNextTransaction
				} else {
					log.DebugDynamic(func() string {
						ruleJson, _ := json.Marshal(rule)
						return fmt.Sprintf("%s %s [%s] [%v] rule value don't match, i: %v, aliasValue: %v, "+
							"subscriberId: %v, memberInfo: %v, rule: %v, ", ruleHelperPrefix, aliasPrefix, method, tx.Payload.TxId, i, aliasValue, aliasValue[rule.Index:rule.Index+rule.Offset], subscriberId, string(ruleJson))
					})
					continue
				}
			}
		}
		// 不匹配返回交易id和读写集
		result <- &commonPb.Transaction{Payload: &commonPb.Payload{TxId: tx.Payload.TxId}, Result: &commonPb.Result{RwSetHash: tx.Result.RwSetHash}}
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
