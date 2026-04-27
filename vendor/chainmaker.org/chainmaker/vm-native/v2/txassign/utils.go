/*
 * Copyright (C) BABEC. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package txassign utils
package txassign

import (
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"fmt"
	"strconv"
)

const (
	nameHeight       = "height"
	nameMethod       = "method"
	nameContractName = "contract_name"
	nameRuleType     = "rule_type"
	nameRule         = "rule"

	errCannotBeNil   = "\"%v\" cannot be nil"
	errCannotBeEmpty = "\"%v\" cannot be empty"
)

func parseParams4height(params map[string][]byte) (height uint64, err error) {
	bytes, ok := params[nameHeight]
	if !ok {
		err = fmt.Errorf(errCannotBeNil, nameHeight)
		return
	}
	height, err = strconv.ParseUint(string(bytes), 10, 64)
	if err != nil {
		return 0, err
	}
	return
}

func parseParams4method(params map[string][]byte) (method string, err error) {
	bytes, ok := params[nameMethod]
	if !ok {
		err = fmt.Errorf(errCannotBeNil, nameMethod)
		return
	}
	method = string(bytes)
	if len(method) == 0 {
		err = fmt.Errorf(errCannotBeEmpty, nameMethod)
		return
	}
	return
}

func parseParams4contractName(params map[string][]byte) (contractName string, err error) {
	bytes, ok := params[nameContractName]
	if !ok {
		err = fmt.Errorf(errCannotBeNil, nameContractName)
		return
	}
	contractName = string(bytes)
	if len(contractName) == 0 {
		err = fmt.Errorf(errCannotBeEmpty, nameContractName)
		return
	}
	return
}

func parseParams4ruleType(params map[string][]byte) (ruleType txassign.RuleType, err error) {
	bytes, ok := params[nameRuleType]
	if !ok {
		err = fmt.Errorf(errCannotBeNil, nameRuleType)
		return
	}
	rt := string(bytes)
	i, err := strconv.Atoi(rt)
	if err != nil {
		err = fmt.Errorf(errCannotBeEmpty, nameRuleType)
		return
	}
	ruleType = txassign.RuleType(i)
	return
}

func parseParams4rule(params map[string][]byte) (rule []byte, err error) {
	rule, ok := params[nameRule]
	if !ok {
		err = fmt.Errorf(errCannotBeNil, nameRule)
		return
	}
	return
}
