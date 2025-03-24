/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper utils
package helper

import (
	"chainmaker.org/chainmaker/utils/v2"
	"fmt"
	"strconv"

	"chainmaker.org/chainmaker/common/v2/bytehelper"
	"chainmaker.org/chainmaker/common/v2/json"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	parameterTrue  = "true"
	parameterFalse = "false"
)

// getRuleType get rule type
func getRuleType(parameters []*commonPb.KeyValuePair) (ruleType txassign.RuleType, err error) {
	parameter, err := getParameterInt(parameters, syscontract.SubscribeBlock_RULE_TYPE.String())
	if parameter == -1 {
		// default type
		return txassign.RuleType_OrgId, nil
	}
	if err != nil {
		return txassign.RuleType_OrgId, err
	}
	ruleType = txassign.RuleType(parameter)
	return
}

// getParameterString Gets a string type parameter from parameters based on key
func getParameterString(parameters []*commonPb.KeyValuePair, key string) (string, error) {
	parameter, err := getParameter(parameters, key)
	if err != nil {
		return "", err
	}
	return string(parameter), nil
}

// GetParameterBool Gets a bool type parameter from parameters based on key
func GetParameterBool(parameters []*commonPb.KeyValuePair, key string) (bool, error) {
	parameter, err := getParameter(parameters, key)
	if err != nil {
		return false, err
	}
	boolString := string(parameter)
	switch boolString {
	case parameterTrue:
		return true, nil
	case parameterFalse:
		return false, nil
	default:
		return false, fmt.Errorf("\"%v\" value: %v", key, boolString)
	}
}

// getParameterInt Gets a int32 type parameter from parameters based on key
func getParameterInt(parameters []*commonPb.KeyValuePair, key string) (int32, error) {
	parameter, err := getParameter(parameters, key)
	if err != nil {
		return -1, err
	}
	i, err := strconv.Atoi(string(parameter))
	if err != nil {
		return -1, err
	}
	return int32(i), nil
}

// GetParameterInt64 Gets a int64 type parameter from parameters based on key
func GetParameterInt64(parameters []*commonPb.KeyValuePair, key string) (int64, error) {
	parameter, err := getParameter(parameters, key)
	if err != nil {
		return -1, err
	}
	return bytehelper.BytesToInt64(parameter)
}

// GetParameter gets a value from the parameters based on the key
func getParameter(parameters []*commonPb.KeyValuePair, key string) ([]byte, error) {
	for _, parameter := range parameters {
		if parameter.Key == key {
			if parameter.Value == nil {
				return nil, fmt.Errorf("\"%v\" cannot be nil", key)
			}
			return parameter.Value, nil
		}
	}
	return nil, fmt.Errorf("\"%v\" is not found", key)
}

// GetParameter gets a value from the parameters based on the key
func getRuleDetail(parameters []*commonPb.KeyValuePair, key string) ([]byte, error) {
	for _, parameter := range parameters {
		if parameter.Key == "rule" {
			if parameter.Value == nil {
				return nil, fmt.Errorf("\"%v\" cannot be nil", key)
			}
			return parameter.Value, nil
		}
	}
	return nil, fmt.Errorf("\"%v\" is not found", key)
}

func checkRules(height uint64, rule *txassign.Rule, logger protocol.Logger, typ string) bool {
	if height >= rule.StartHeight && (height <= rule.EndHeight || 0 == rule.EndHeight) {
		if rule.Status == txassign.RuleStatus_Enabled {
			return true
		} else {
			// Rule not enabled
			logger.DebugDynamic(func() string {
				bytes, _ := json.Marshal(rule)
				return fmt.Sprintf("[RuleHelper] %v rule not enabled, rule: %v", typ, string(bytes))
			})
		}
	} else {
		// Height is not within the rules
		logger.DebugDynamic(func() string {
			bytes, _ := json.Marshal(rule)
			return fmt.Sprintf("[RuleHelper] %v height is not within the rules, current: %v, start: %v, "+
				"end: %v, rule: %v", typ, height, rule.StartHeight, rule.EndHeight, string(bytes))
		})
	}
	return false
}

func DispatchTxVerifyTask(txCount int) [][]int {
	batchCount := utils.CalcTxVerifyWorkers(txCount)
	var batchIndex = make([][]int, batchCount)
	batchSize := txCount / batchCount
	for i := 0; i < batchCount-1; i++ {
		batchIndex[i] = []int{i * batchSize, i*batchSize + batchSize}
	}
	batchIndex[batchCount-1] = []int{(batchCount - 1) * batchSize, txCount}
	return batchIndex
}
