/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper alias test
package subscribefilter

import (
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	"chainmaker.org/chainmaker/net-liquid/logger"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"fmt"
	"github.com/golang/mock/gomock"
	"reflect"
	"testing"

	"chainmaker.org/chainmaker/pb-go/v2/txassign"
)

const (
	contractName = "contractName"
	method       = "methods"
)

var (
	end   []byte
	start []byte
	rule  = &txassign.FilterRule{Id: txassign.RuleType_Alias}
)

func getHelper(t *testing.T, role protocol.Role, cacheData bool) *BaseHelper {
	helper, err := newBaseHelper(&commonPb.Transaction{
		Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
			{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: end},
			{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: start},
		}},
	}, func() protocol.BlockchainStore {
		store := mock.NewMockBlockchainStore(gomock.NewController(t))
		store.EXPECT().GetLastBlock().AnyTimes().Return(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 1}}, nil)
		return store
	}(), role, logger.NewLogPrinter("TEST"))
	if err != nil {
		panic(err)
	}
	if cacheData {
		helper.ruleCache.Put(1, "/rule/1/contractName/methods", rule)
	}
	return helper
}

func init() {
	var err error
	end, err = bytehelper.Int64ToBytes(1)
	if err != nil {
		panic(err)
	}
	start, err = bytehelper.Int64ToBytes(-1)
	if err != nil {
		panic(err)
	}
}
func TestAliasHelper_FilterRule(t *testing.T) {
	type fields struct {
		helper       *BaseHelper
		contractName string
		methods      []string
	}
	type args struct {
		cache bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *txassign.FilterRule
		wantErr bool
	}{
		{
			name: "正常流",
			fields: fields{
				helper: func() *BaseHelper {
					helper, err := newBaseHelper(&commonPb.Transaction{
						Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
							{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: end},
							{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: start},
						}},
					}, func() protocol.BlockchainStore {
						store := mock.NewMockBlockchainStore(gomock.NewController(t))
						store.EXPECT().GetLastBlock().AnyTimes().Return(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 1}}, nil)
						return store
					}(), protocol.RoleAdmin, logger.NewLogPrinter("TEST"))
					if err != nil {
						t.Errorf("%v", err)
						return nil
					}
					helper.ruleCache.Put(1, "/rule/1/contractName/methods", rule)
					return helper
				}(),
				contractName: contractName,
				methods:      []string{method},
			},
			args:    args{cache: true},
			want:    rule,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AliasHelper{
				helper:       tt.fields.helper,
				contractName: tt.fields.contractName,
				methods:      tt.fields.methods,
			}
			got, err := h.FilterRule(tt.args.cache)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterRule() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAliasHelper_GetBaseHelper(t *testing.T) {
	helper := getHelper(t, protocol.RoleLight, true)
	type fields struct {
		helper       *BaseHelper
		contractName string
		methods      []string
	}
	tests := []struct {
		name   string
		fields fields
		want   *BaseHelper
	}{
		{
			name: "正常流",
			fields: fields{
				helper: helper,
			},
			want: helper,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AliasHelper{
				helper:       tt.fields.helper,
				contractName: tt.fields.contractName,
				methods:      tt.fields.methods,
			}
			if got := h.GetBaseHelper(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBaseHelper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAliasHelper_GetType(t *testing.T) {
	type fields struct {
		helper       *BaseHelper
		contractName string
		methods      []string
	}
	tests := []struct {
		name   string
		fields fields
		want   txassign.RuleType
	}{
		{
			name:   "正常流",
			fields: fields{},
			want:   txassign.RuleType_Alias,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AliasHelper{
				helper:       tt.fields.helper,
				contractName: tt.fields.contractName,
				methods:      tt.fields.methods,
			}
			if got := h.GetType(); got != tt.want {
				t.Errorf("GetType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAliasHelper_Validate(t *testing.T) {
	helper := getHelper(t, protocol.RoleLight, true)
	type fields struct {
		helper       *BaseHelper
		contractName string
		methods      []string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "异常流 contractName cannot empty",
			fields: fields{
				helper:       helper,
				contractName: "",
			},
			wantErr: true,
		},
		{
			name: "异常流 methods cannot empty",
			fields: fields{
				helper:  helper,
				methods: []string{},
			},
			wantErr: true,
		},
		{
			name: "异常流 methods cannot empty",
			fields: fields{
				helper:       helper,
				contractName: contractName,
				methods:      []string{method},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AliasHelper{
				helper:       tt.fields.helper,
				contractName: tt.fields.contractName,
				methods:      tt.fields.methods,
			}
			if err := h.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAliasHelper_Verify(t *testing.T) {
	type fields struct {
		helper       *BaseHelper
		contractName string
		methods      []string
	}
	type args struct {
		current *commonPb.Block
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult []*commonPb.Transaction
	}{
		{
			name: "正常流 non-light nodes do not judge rules",
			fields: fields{
				helper:       getHelper(t, protocol.RoleAdmin, false),
				contractName: contractName,
				methods:      []string{method},
			},
			args:       args{current: &commonPb.Block{Txs: nil}},
			wantResult: nil,
		},
		{
			name: "正常流 规则不存在",
			fields: fields{
				helper:       getHelper(t, protocol.RoleAdmin, false),
				contractName: contractName,
				methods:      []string{method},
			},
			args:       args{current: &commonPb.Block{Txs: nil}},
			wantResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//h := &AliasHelper{
			//	helper:       tt.fields.helper,
			//	contractName: tt.fields.contractName,
			//	methods:      tt.fields.methods,
			//}
			//if gotResult, _ := h.Verify(tt.args.current); !reflect.DeepEqual(gotResult, tt.wantResult) {
			//	t.Errorf("Verify() = %v, want %v", gotResult, tt.wantResult)
			//}
		})
	}
}

func TestAliasHelper_getKey(t *testing.T) {
}

func Test_getParameters(t *testing.T) {
	type args struct {
		parameters []*commonPb.KeyValuePair
	}
	tests := []struct {
		name             string
		args             args
		wantContractName string
		wantMethod       string
		wantErr          bool
	}{
		{
			name: "正常流",
			args: args{parameters: []*commonPb.KeyValuePair{
				{Key: syscontract.SubscribeBlock_CONTRACT_NAME.String(), Value: []byte(contractName)},
				{Key: syscontract.SubscribeBlock_METHOD.String(), Value: []byte(method)},
			}},
			wantContractName: contractName,
			wantMethod:       method,
			wantErr:          false,
		},
		{
			name: "异常流",
			args: args{parameters: []*commonPb.KeyValuePair{
				{Key: syscontract.SubscribeBlock_CONTRACT_NAME.String()},
			}},
			wantContractName: "",
			wantMethod:       "",
			wantErr:          true,
		},
		{
			name: "异常流",
			args: args{parameters: []*commonPb.KeyValuePair{
				{Key: syscontract.SubscribeBlock_METHOD.String()},
			}},
			wantContractName: "",
			wantMethod:       "",
			wantErr:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContractName, gotMethod, err := getParameters(tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("getParameters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotContractName != tt.wantContractName {
				t.Errorf("getParameters() gotContractName = %v, want %v", gotContractName, tt.wantContractName)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("getParameters() gotMethod = %v, want %v", gotMethod, tt.wantMethod)
			}
		})
	}
}

func Test_newAliasHelper(t *testing.T) {
	var m = make(map[string]*txassign.AliasRule)
	m["ddd"] = &txassign.AliasRule{
		Rule:   nil,
		Index:  1,
		Offset: 2,
		Name:   "fsdafsadfdsa",
	}
	fmt.Println(fmt.Sprintf("%v", m))
}
