/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper utils test
package subscribefilter

import (
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	"chainmaker.org/chainmaker/net-liquid/logger"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

const (
	keyParameter   = "key"
	valueParameter = "value"
)

var (
	valueParameterByte = []byte(valueParameter)
)

type args struct {
	parameters []*commonPb.KeyValuePair
	key        string
}

func Test_getParameter(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "异常流 key cannot be nil",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "异常流 key is not found",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter}},
				key:        keyParameter,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: valueParameterByte}},
				key:        keyParameter,
			},
			want:    valueParameterByte,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getParameter(tt.args.parameters, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParameter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getParameter() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParameterBool(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "异常流 key cannot be nil",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "异常流 value not mach",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: valueParameterByte}},
				key:        keyParameter,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: []byte(parameterTrue)}},
				key:        keyParameter,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: []byte(parameterFalse)}},
				key:        keyParameter,
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetParameterBool(tt.args.parameters, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetParameterBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetParameterBool() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParameterInt64(t *testing.T) {
	int64bytes, err := bytehelper.Int64ToBytes(1)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "异常流 key cannot be nil",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: int64bytes}},
				key:        keyParameter,
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetParameterInt64(tt.args.parameters, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetParameterInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetParameterInt64() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getParameterInt(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		want    int32
		wantErr bool
	}{
		{
			name: "异常流 key cannot be nil",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: []byte("1")}},
				key:        keyParameter,
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getParameterInt(tt.args.parameters, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParameterInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getParameterInt() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getParameterString(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "异常流 key cannot be nil",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: keyParameter, Value: valueParameterByte}},
				key:        keyParameter,
			},
			want:    valueParameter,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getParameterString(tt.args.parameters, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParameterString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getParameterString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRuleType(t *testing.T) {
	tests := []struct {
		name         string
		args         args
		wantRuleType txassign.RuleType
		wantErr      bool
	}{
		{
			name: "正常流 转换失败走orgid",
			args: args{
				parameters: nil,
				key:        keyParameter,
			},
			wantRuleType: txassign.RuleType_OrgId,
			wantErr:      false,
		},
		{
			name: "正常流 转换失败走orgid",
			args: args{
				parameters: []*commonPb.KeyValuePair{{Key: syscontract.SubscribeBlock_RULE_TYPE.String(), Value: []byte("1")}},
				key:        keyParameter,
			},
			wantRuleType: txassign.RuleType_Alias,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRuleType, err := getRuleType(tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRuleType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRuleType != tt.wantRuleType {
				t.Errorf("getRuleType() gotRuleType = %v, want %v", gotRuleType, tt.wantRuleType)
			}
		})
	}
}

func Test_checkRules(t *testing.T) {
	type args struct {
		height uint64
		rule   *txassign.Rule
		logger protocol.Logger
		typ    string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "正常流 高度在范围内 启动规则",
			args: args{
				height: 20,
				rule: &txassign.Rule{
					Status:      txassign.RuleStatus_Enabled,
					StartHeight: 0,
					EndHeight:   0,
				},
				logger: nil,
				typ:    "",
			},
			want: true,
		},
		{
			name: "正常流 高度在范围内 不启动规则",
			args: args{
				height: 20,
				rule: &txassign.Rule{
					Status:      txassign.RuleStatus_Disable,
					StartHeight: 0,
					EndHeight:   0,
				},
				logger: logger.NewLogPrinter("TEST"),
				typ:    "",
			},
			want: false,
		},
		{
			name: "正常流 高度不在范围内",
			args: args{
				height: 20,
				rule: &txassign.Rule{
					Status:      txassign.RuleStatus_Disable,
					StartHeight: 0,
					EndHeight:   19,
				},
				logger: logger.NewLogPrinter("TEST"),
				typ:    "",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkRules(tt.args.height, tt.args.rule, tt.args.logger, tt.args.typ); got != tt.want {
				t.Errorf("checkRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDispatchTxVerifyTask(t *testing.T) {
	type args struct {
		txCount int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "10000",
			args: args{txCount: 10000},
			want: 100,
		},
		{
			name: "1000",
			args: args{txCount: 1000},
			want: 10,
		},
		{
			name: "500",
			args: args{txCount: 500},
			want: 10,
		},
		{
			name: "100",
			args: args{txCount: 100},
			want: 5,
		},
		{
			name: "50",
			args: args{txCount: 50},
			want: 5,
		},
		{
			name: "1",
			args: args{txCount: 1},
			want: 1,
		},
		{
			name: "2",
			args: args{txCount: 2},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchIndex := DispatchTxVerifyTask(tt.args.txCount)
			for i, index := range batchIndex {
				fmt.Println("batch:", i, "start:", index[0], "end:", index[1])
			}
			assert.Equalf(t, tt.want, len(batchIndex), "DispatchTxVerifyTask(%v)", tt.args.txCount)
		})
	}
}
