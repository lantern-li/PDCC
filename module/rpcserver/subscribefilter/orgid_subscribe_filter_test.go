/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper orgid test
package subscribefilter

import (
	"chainmaker.org/chainmaker/net-liquid/logger"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOrgIdHelper_FilterRule(t *testing.T) {
	filter := &txassign.FilterRule{Id: txassign.RuleType_Alias}
	type fields struct {
		filterManager *SubscribeFilterManager
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
				filterManager: func() *SubscribeFilterManager {
					helper, err := newSubscribeFilterManager(&commonPb.Transaction{
						Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
							{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: end},
							{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: start},
						}},
					}, func() protocol.BlockchainStore {
						store := mock.NewMockBlockchainStore(gomock.NewController(t))
						store.EXPECT().GetLastBlock().AnyTimes().Return(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 1}}, nil)
						bayes, err := proto.Marshal(filter)
						if err != nil {
							t.Error(err)
							return nil
						}
						store.EXPECT().ReadObject(gomock.Any(), gomock.Any()).AnyTimes().Return(bayes, nil)
						return store
					}(), protocol.RoleAdmin, logger.NewLogPrinter("TEST"))
					if err != nil {
						t.Errorf("%v", err)
						return nil
					}
					return helper
				}(),
			},
			args:    args{cache: true},
			want:    rule,
			wantErr: false,
		},
		{
			name: "正常流",
			fields: fields{
				filterManager: func() *SubscribeFilterManager {
					helper, err := newSubscribeFilterManager(&commonPb.Transaction{
						Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
							{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: end},
							{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: start},
						}},
					}, func() protocol.BlockchainStore {
						store := mock.NewMockBlockchainStore(gomock.NewController(t))
						store.EXPECT().GetLastBlock().AnyTimes().Return(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 1}}, nil)
						store.EXPECT().ReadObject(gomock.Any(), gomock.Any()).AnyTimes().Return(nil, nil)
						return store
					}(), protocol.RoleAdmin, logger.NewLogPrinter("TEST"))
					if err != nil {
						t.Errorf("%v", err)
						return nil
					}
					return helper
				}(),
			},
			args: args{cache: false},
			want: &txassign.FilterRule{
				Id: txassign.RuleType_OrgId,
				OrgId: []*txassign.OrgIdRule{{Rule: &txassign.Rule{
					Status:      txassign.RuleStatus_Enabled,
					StartHeight: 0,
					EndHeight:   0,
				}}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &OrgIdSubscriberFilter{
				filterManager: tt.fields.filterManager,
			}

			got, err := h.FilterRule(tt.args.cache)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "FilterRule(%v)", tt.args.cache)
		})
	}
}

func TestOrgIdHelper_GetBaseHelper(t *testing.T) {
}

func TestOrgIdHelper_GetType(t *testing.T) {
	type fields struct {
		filterManager *SubscribeFilterManager
	}
	tests := []struct {
		name   string
		fields fields
		want   txassign.RuleType
	}{
		{
			name:   "正常流",
			fields: fields{filterManager: &SubscribeFilterManager{}},
			want:   txassign.RuleType_OrgId,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &OrgIdSubscriberFilter{
				filterManager: tt.fields.filterManager,
			}
			assert.Equalf(t, tt.want, h.GetType(), "GetType()")
		})
	}
}

// see Test_baseHelper_validate
func TestOrgIdHelper_Validate(t *testing.T) {
}

func TestOrgIdHelper_Verify(t *testing.T) {
	type fields struct {
		helper *SubscribeFilterManager
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//h := &OrgIdHelper{
			//	helper: tt.fields.helper,
			//}
			//result, _ := h.Verify(tt.args.current)
			//assert.Equalf(t, tt.wantResult, result, "Verify(%v)", tt.args.current)
		})
	}
}

func TestOrgIdHelper_getKey(t *testing.T) {
}

func Test_newOrgIdHelper(t *testing.T) {
}
