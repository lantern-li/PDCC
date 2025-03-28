/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package subscribefilter subscribe filter
package subscribefilter

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

// AliasSubscribeFilter alias filter
type AliasSubscribeFilter struct {
	// SubscribeFilterManager sub filter manager
	filterManager *SubscribeFilterManager
	// contractName contract name
	contractName string
	// contract methods
	methods []string
	// subscriberId 订阅者（别名ID）
	subscriberId *IdentityCode

	pool *ants.Pool
}

func (f *AliasSubscribeFilter) GetStore() protocol.BlockchainStore {
	return f.filterManager.store
}

func (f *AliasSubscribeFilter) GetType() txassign.RuleType {
	return txassign.RuleType_Alias
}

// newAliasSubscribeFilter new filter instance
func newAliasSubscribeFilter(tx *commonPb.Transaction, store protocol.BlockchainStore, role protocol.Role, log protocol.Logger, pool *ants.Pool) (*AliasSubscribeFilter, error) {
	contractName, method, err := getParameters(tx.Payload.Parameters)
	if err != nil {
		return nil, err
	}
	filterManager, err := newSubscribeFilterManager(tx, store, role, log)
	if err != nil {
		return nil, err
	}
	subscriberId, err := GetIdentityCode(string(tx.Sender.Signer.MemberInfo))
	if err != nil {
		return nil, fmt.Errorf("%v, subscriberId: %v", err, string(tx.Sender.Signer.MemberInfo))
	}

	return &AliasSubscribeFilter{
		contractName:  contractName,
		methods:       strings.Split(method, sep),
		filterManager: filterManager,
		subscriberId:  subscriberId,
		pool:          pool,
	}, nil
}

// GetSubscribeFilterManagement get subscribe filter management
func (f *AliasSubscribeFilter) GetSubscribeFilterManagement() *SubscribeFilterManager {
	return f.filterManager
}

func (f *AliasSubscribeFilter) GetSubscriber() *IdentityCode {
	return f.subscriberId
}

// FilterTxs filter txs by block
func (f *AliasSubscribeFilter) FilterTxs(current *commonPb.Block, withTxId bool) (result []*commonPb.Transaction, count int) {
	txs := current.Txs
	height := current.Header.BlockHeight
	filterRules, err := f.FilterRule(true)
	if err != nil {
		f.filterManager.Log.Errorf("%s %s get filter rules fail, error: %v", ruleHelperPrefix, aliasPrefix, err)
		return
	}
	if len(filterRules) != len(f.methods) {
		f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s no rules available, methods: %v, filterRules: %v", ruleHelperPrefix, aliasPrefix, f.methods, filterRules))
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
			if checkRules(height, rule0.Rule, f.filterManager.Log, aliasPrefix) {
				rules[method] = rule0
				break
			}
		}
	}
	if len(rules) != len(f.methods) {
		f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("%s %s no rules available, ", ruleHelperPrefix, aliasPrefix))
		return
	}

	var (
		wg            = &sync.WaitGroup{}
		matchTxsIndex = make([]bool, current.Header.TxCount)
	)
	batchIndexes := DispatchTxVerifyTask(int(current.Header.TxCount))
	total := len(batchIndexes)
	for _, method := range f.methods {
		for i, index := range batchIndexes {
			wg.Add(1)
			batch := i
			startIndex := index[0]
			endIndex := index[1]
			method1 := method
			f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("[%v] run batch, total:%v,batch:%v,startIndex:%v,endIndex:%v", height, total, batch, startIndex, endIndex))
			err = f.pool.Submit(func() {
				verifyTxs(wg, f.filterManager, height, rules[method1], txs, method1, f.contractName, f.subscriberId, matchTxsIndex, total, batch, startIndex, endIndex)
			})
			if err != nil {
				f.filterManager.Log.Errorf("subscribe pool submit fail. error: %v", err)
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

func verifyTxs(
	wg *sync.WaitGroup,
	filterManager *SubscribeFilterManager,
	height uint64,
	rule *txassign.AliasRule,
	txs []*commonPb.Transaction,
	method, contractName string,
	subscriberId *IdentityCode,
	matchTxsIndex []bool,
	total, batch, startIndex, endIndex int,
) {
	defer wg.Done()

	filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc(
		"run batch, total:%v, batch:%v, startIndex:%v, endIndex:%v", total, batch, startIndex, endIndex))

	for index := startIndex; index < endIndex; index++ {
		tx := txs[index]

		// 更新 rule 缓存
		if err := filterManager.updateAliasRuleCache(tx, height); err != nil {
			filterManager.Log.Errorf("updateAliasRuleCache error: %v", err.Error())
		}

		// 合约名与方法不匹配时跳过该笔交易
		if contractName != tx.Payload.ContractName || method != tx.Payload.Method {
			filterManager.Log.DebugDynamic(func() string {
				ruleBytes, _ := json.Marshal(rule)
				return fmt.Sprintf("%s %s [%s] [%v] [%v] contract name or methods do not match, "+
					"contract_name(tx:%v, rule:%v), methods(tx:%v, rule:%v), rule: %v",
					ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId,
					tx.Payload.ContractName, contractName, tx.Payload.Method, method, string(ruleBytes))
			})
			continue
		}

		// 判断该笔交易是否匹配当前规则
		if isTxMatched(tx, rule, method, height, filterManager, subscriberId) {
			matchTxsIndex[index] = true
		}
	}
}

// isTxMatched 判断单笔交易是否满足 AliasRule 的匹配要求
func isTxMatched(
	tx *commonPb.Transaction,
	rule *txassign.AliasRule,
	method string,
	height uint64,
	filterManager *SubscribeFilterManager,
	subscriberId *IdentityCode,
) bool {
	// 遍历 rule.Name 中的每个字段名称
	for _, name := range strings.Split(rule.Name, sep) {
		// 根据 rule 字段名称从交易参数中获取对应的字符串值
		aliasValueString, err := getParameterString(tx.Payload.Parameters, name)
		if err != nil {
			filterManager.Log.DebugDynamic(func() string {
				ruleJSON, _ := json.Marshal(rule)
				return fmt.Sprintf("%s %s [%s] [%v] [%v] failed to get parameter string for rule name, rule: %v, error: %v",
					ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, string(ruleJSON), err)
			})
			// 获取失败，跳过当前字段（继续检查其他字段）
			continue
		}

		// 检查 rule.Index + rule.Offset 是否超出 aliasValueString 长度
		// todo index和offset的设计目的暂不明确
		if rule.Index+rule.Offset > uint32(len(aliasValueString)) {
			filterManager.Log.DebugDynamic(func() string {
				ruleJSON, _ := json.Marshal(rule)
				return fmt.Sprintf("%s %s [%s] [%v] [%v] rule value bounds out of range, value: %v, rule: %v",
					ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, aliasValueString, string(ruleJSON))
			})
			// 若越界，跳过当前字段（继续检查其他字段）
			continue
		}

		// 获取该字段多个别名值
		aliasValues := strings.Split(aliasValueString, sep)
		for i, aliasValue := range aliasValues {
			participantId, err := GetIdentityCode(aliasValue)
			if err != nil {
				filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc(
					"GetIdentityCode [%v] error: %v", aliasValue, err.Error()))
				// 出现错误这里不return，继续执行，以便于让AdminCode codeType类型的订阅能订阅到该交易
			}
			// 判断 subscriberId 与 participantId 是否匹配
			if subscriberId.Match(participantId) {
				filterManager.Log.DebugDynamic(func() string {
					ruleJSON, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] [%v] rule value match, index: %v, aliasValue: %v, subscriberId: %v, rule: %v",
						ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, i, aliasValue, subscriberId, string(ruleJSON))
				})
				return true
			} else {
				filterManager.Log.DebugDynamic(func() string {
					ruleJSON, _ := json.Marshal(rule)
					return fmt.Sprintf("%s %s [%s] [%v] [%v] rule value does not match, index: %v, aliasValue: %v, subscriberId: %v, rule: %v",
						ruleHelperPrefix, aliasPrefix, method, height, tx.Payload.TxId, i, aliasValue, subscriberId, string(ruleJSON))
				})
			}
		}
	}
	return false
}

// getKey get key
func (f *AliasSubscribeFilter) getKey(method string) string {
	format := strconv.FormatInt(int64(txassign.RuleType_Alias), 10)
	return path.Join(RulePrefix, format, f.contractName, method)
}

// FilterRule get filter rule
func (f *AliasSubscribeFilter) FilterRule(cache bool) (map[string]*txassign.FilterRule, error) {
	filterRules := make(map[string]*txassign.FilterRule, len(f.methods))
	for i, method := range f.methods {
		key := f.getKey(method)
		f.filterManager.Log.DebugDynamic(filtercommon.LoggingFixLengthFunc("get filter rule with key, %v", key))
		filterRule, err := f.filterManager.getFilterRule(key, cache)
		if err != nil {
			f.filterManager.Log.Errorf("filter rule not found. key: %v, i: %v", key, i)
			continue
		}
		filterRules[method] = filterRule
	}
	return filterRules, nil
}

// Validate parameters
func (f *AliasSubscribeFilter) Validate() error {
	err := f.filterManager.validate()
	if err != nil {
		return err
	}
	if len(f.contractName) == 0 {
		return errors.New("contractName cannot empty")
	}
	if len(f.methods) == 0 {
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
