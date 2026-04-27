/*
 * Copyright (C) BABEC. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package txassign
package txassign

import (
	"errors"
	"fmt"

	"chainmaker.org/chainmaker/common/v2/json"
	"chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
)

var (
	contractName = syscontract.SystemContract_TX_ASSIGN.String()
)

func NewContract(log protocol.Logger) *Contract {
	return &Contract{
		log:     log,
		methods: registerTxAssignContractMethods(log),
	}
}

func registerTxAssignContractMethods(log protocol.Logger) map[string]common.ContractFunc {
	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &Runtime{log: log}

	methodMap[syscontract.TxAssignFunction_RegisterRule.String()] = common.WrapResultFunc(runtime.RegisterRule)
	methodMap[syscontract.TxAssignFunction_GetRules.String()] = common.WrapResultFunc(runtime.GetRules)
	methodMap[syscontract.TxAssignFunction_GetLastRule.String()] = common.WrapResultFunc(runtime.GetLastRule)
	methodMap[syscontract.TxAssignFunction_UpdateRuleByHeight.String()] = common.WrapResultFunc(runtime.UpdateRuleByHeight)

	return methodMap
}

// Contract transaction assign
type Contract struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

func (c *Contract) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

type Runtime struct {
	log protocol.Logger
}

/*
RegisterRule
params:

	ruleType int 过滤规则类型
	rule []byte 过滤规则详细内容
*/
func (r *Runtime) RegisterRule(context protocol.TxSimContext, params map[string][]byte) ([]byte, error) {
	admin, err := isAdminByCtx(context)
	if err != nil {
		return nil, fmt.Errorf("check permission fail, error: %v", err)
	}
	if !admin {
		return nil, errors.New("signer no permission")
	}

	ruleType, err := parseParams4ruleType(params)
	if err != nil {
		return nil, err
	}
	ruleBytes, err := parseParams4rule(params)
	if err != nil {
		return nil, err
	}
	ruleHelper, err := newHelper4byte(ruleType, ruleBytes, context)
	if err != nil {
		return nil, err
	}
	filterRule, err := ruleHelper.getFilterRule()
	if err != nil {
		return nil, err
	}
	if filterRule == nil {
		filterRule = ruleHelper.initFilterRule(filterRule)
	} else {
		filterRule = ruleHelper.appendFilterRule(filterRule)
	}
	err = ruleHelper.saveFilterRule(filterRule)
	if err != nil {
		return nil, err
	}
	return []byte{}, nil
}

// GetRules 获取设置的交易过滤规则
func (r *Runtime) GetRules(context protocol.TxSimContext, params map[string][]byte) ([]byte, error) {
	admin, err := isAdminByCtx(context)
	if err != nil {
		return nil, fmt.Errorf("check permission fail, error: %v", err)
	}
	if !admin {
		return nil, errors.New("signer no permission")
	}

	ruleType, err := parseParams4ruleType(params)
	if err != nil {
		return nil, err
	}
	contractName0, err := parseParams4contractName(params)
	if err != nil {
		return nil, err
	}
	method, err := parseParams4method(params)
	if err != nil {
		return nil, err
	}
	ruleHelper, err := newHelper4rule(ruleType, context, contractName0, method)
	if err != nil {
		return nil, err
	}
	rule, err := ruleHelper.getFilterRule()
	if err != nil {
		return nil, err
	}
	ruleBytes, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return ruleBytes, nil
}

// GetLastRule get last rule
func (r *Runtime) GetLastRule(context protocol.TxSimContext, params map[string][]byte) ([]byte, error) {
	admin, err := isAdminByCtx(context)
	if err != nil {
		return nil, fmt.Errorf("check permission fail, error: %v", err)
	}
	if !admin {
		return nil, errors.New("signer no permission")
	}

	ruleType, err := parseParams4ruleType(params)
	if err != nil {
		return nil, err
	}
	contractName0, err := parseParams4contractName(params)
	if err != nil {
		return nil, err
	}
	method, err := parseParams4method(params)
	if err != nil {
		return nil, err
	}
	ruleHelper, err := newHelper4rule(ruleType, context, contractName0, method)
	if err != nil {
		return nil, err
	}
	filterRule, err := ruleHelper.getFilterRule()
	if err != nil {
		return nil, err
	}
	if filterRule == nil {
		return nil, nil
	}
	rule, err := ruleHelper.getLastRule(filterRule)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

// UpdateRuleByHeight update rule by height
func (r *Runtime) UpdateRuleByHeight(context protocol.TxSimContext, params map[string][]byte) ([]byte, error) {
	admin, err := isAdminByCtx(context)
	if err != nil {
		return nil, fmt.Errorf("check permission fail, error: %v", err)
	}
	if !admin {
		return nil, errors.New("signer no permission")
	}

	ruleType, err := parseParams4ruleType(params)
	if err != nil {
		return nil, err
	}
	ruleBytes, err := parseParams4rule(params)
	if err != nil {
		return nil, err
	}
	height, err := parseParams4height(params)
	if err != nil {
		return nil, err
	}
	ruleHelper, err := newHelper4byte(ruleType, ruleBytes, context)
	if err != nil {
		return nil, err
	}

	err = ruleHelper.rangeUpdate(func(i int, rule *txassign.Rule) bool {
		if rule.StartHeight == height {
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func isAdminByCtx(context protocol.TxSimContext) (bool, error) {
	control, err := context.GetAccessControl()
	if err != nil {
		return false, err
	}
	tx := context.GetTx()
	var admin bool
	// 检查背书是否是admin权限
	for _, endorser := range tx.GetEndorsers() {
		result, err := isAdminByMember(control, endorser.Signer)
		if err != nil {
			return false, err
		}
		if result {
			admin = true
		}
	}
	if admin {
		return admin, nil
	}
	// 检查发送方是否是admin权限
	return isAdminByMember(control, tx.GetSender().Signer)
}

func isAdminByMember(control protocol.AccessControlProvider, member *accesscontrol.Member) (bool, error) {
	m, err := control.NewMember(member)
	if err != nil {
		return false, err
	}
	if m.GetRole() != protocol.RoleAdmin {
		return false, nil
	}
	return true, nil
}
