/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper rule cache test
package helper

import (
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestFactory(t *testing.T) {
	filter := &txassign.FilterRule{
		Id: txassign.RuleType_OrgId,
		OrgId: []*txassign.OrgIdRule{{Rule: &txassign.Rule{
			Status:      txassign.RuleStatus_Enabled,
			StartHeight: 0,
			EndHeight:   0,
		}}},
		Alias:   nil,
		UserIds: nil,
		OrgIds:  nil,
	}
	const (
		height = 1
	)

	Factory().cache.Put(height, keyParameter, filter)
	got, got1 := Factory().cache.Get(keyParameter)
	assert.True(t, reflect.DeepEqual(got, filter), "result not match")
	assert.True(t, got1 == height, "height not match")
}
