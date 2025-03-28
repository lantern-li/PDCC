/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper base test
package subscribefilter

import (
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	"chainmaker.org/chainmaker/net-liquid/logger"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"errors"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewFilterManagement(t *testing.T) {
	bytes, err := bytehelper.Int64ToBytes(1)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	store := func() protocol.BlockchainStore {
		store := mock.NewMockBlockchainStore(gomock.NewController(t))
		store.EXPECT().GetLastBlock().AnyTimes().Return(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 1}}, nil)
		return store
	}()
	type args struct {
		tx    *commonPb.Transaction
		store protocol.BlockchainStore
		role  protocol.Role
		log   protocol.Logger
	}
	tests := []struct {
		name    string
		args    args
		want    *SubscribeFilterManager
		wantErr bool
	}{
		{
			name: "异常流 start_blcok is not found",
			args: args{
				tx: &commonPb.Transaction{
					Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{{Key: syscontract.SubscribeBlock_START_BLOCK.String()}}},
				},
				store: nil,
				role:  "",
				log:   nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "异常流 end_blcok is not found",
			args: args{
				tx: &commonPb.Transaction{
					Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{{Key: syscontract.SubscribeBlock_END_BLOCK.String()}}},
				},
				store: nil,
				role:  "",
				log:   nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "异常流 store error",
			args: args{
				tx: &commonPb.Transaction{
					Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: bytes}}},
				},
				store: func() protocol.BlockchainStore {
					store := mock.NewMockBlockchainStore(gomock.NewController(t))
					store.EXPECT().GetLastBlock().AnyTimes().Return(nil, errors.New("store error"))
					return store
				}(),
				role: "",
				log:  nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "正常流",
			args: args{
				tx: &commonPb.Transaction{
					Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
						{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: bytes},
						{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: bytes},
					}},
				},
				store: store,
				role:  protocol.RoleAdmin,
				log:   logger.NewLogPrinter("TEST"),
			},
			want: func() *SubscribeFilterManager {
				helper, err := newSubscribeFilterManager(&commonPb.Transaction{
					Payload: &commonPb.Payload{Parameters: []*commonPb.KeyValuePair{
						{Key: syscontract.SubscribeBlock_END_BLOCK.String(), Value: bytes},
						{Key: syscontract.SubscribeBlock_START_BLOCK.String(), Value: bytes},
					}},
				}, store, protocol.RoleAdmin, logger.NewLogPrinter("TEST"))
				if err != nil {
					return nil
				}
				return helper
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSubscribeFilterManager(tt.args.tx, tt.args.store, tt.args.role, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("newBaseHelper() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "newBaseHelper(%v, %v, %v, %v)", tt.args.tx, tt.args.store, tt.args.role, tt.args.log)
		})
	}
}

func TestNewHelper(t *testing.T) {
}

func Test_baseHelper_getFilterRule(t *testing.T) {
	nilFilterRule := (*txassign.FilterRule)(nil)
	filterRule := &txassign.FilterRule{}
	type fields struct {
		Start           int64
		End             int64
		LastBlockHeight uint64
		Log             protocol.Logger
		Tx              *commonPb.Transaction
		store           protocol.BlockchainStore
		ruleCache       *ruleCache
		role            protocol.Role
	}
	type args struct {
		key   string
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
			name: "正常流 通过缓存获得交易分发过滤器",
			fields: fields{
				ruleCache: func() *ruleCache {
					cache := Factory().cache
					cache.Put(1, keyParameter, filterRule)
					return Factory().cache
				}(),
			},
			args: args{
				key:   keyParameter,
				cache: true,
			},
			want:    filterRule,
			wantErr: false,
		},
		{
			name: "异常流 通过存储获得交易分发过滤器 error",
			fields: fields{
				ruleCache: Factory().cache,
				store: func() protocol.BlockchainStore {
					store := mock.NewMockBlockchainStore(gomock.NewController(t))
					store.EXPECT().ReadObject(gomock.Any(), gomock.Any()).AnyTimes().Return(nil, errors.New("store error"))
					return store
				}(),
			},
			args: args{
				key:   keyParameter,
				cache: false,
			},
			want:    nilFilterRule,
			wantErr: true,
		},
		{
			name: "异常流 通过存储获得交易分发过滤器 nil",
			fields: fields{
				ruleCache: Factory().cache,
				store: func() protocol.BlockchainStore {
					store := mock.NewMockBlockchainStore(gomock.NewController(t))
					store.EXPECT().ReadObject(gomock.Any(), gomock.Any()).AnyTimes().Return(nil, nil)
					return store
				}(),
			},
			args: args{
				key:   keyParameter,
				cache: false,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "正常流 通过存储获得交易分发过滤器 nil",
			fields: fields{
				ruleCache:       Factory().cache,
				LastBlockHeight: 20,
				store: func() protocol.BlockchainStore {
					store := mock.NewMockBlockchainStore(gomock.NewController(t))
					bayes, err := proto.Marshal(filterRule)
					if err != nil {
						t.Errorf("%v", err)
						return nil
					}
					store.EXPECT().ReadObject(gomock.Any(), gomock.Any()).AnyTimes().Return(bayes, nil)
					return store
				}(),
			},
			args: args{
				key:   keyParameter,
				cache: false,
			},
			want:    filterRule,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &SubscribeFilterManager{
				Start:           tt.fields.Start,
				End:             tt.fields.End,
				LastBlockHeight: tt.fields.LastBlockHeight,
				Log:             tt.fields.Log,
				Tx:              tt.fields.Tx,
				store:           tt.fields.store,
				ruleCache:       tt.fields.ruleCache,
				role:            tt.fields.role,
			}
			got, err := h.getFilterRule(tt.args.key, tt.args.cache)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFilterRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "getFilterRule(%v, %v)", tt.args.key, tt.args.cache)
		})
	}
}

func Test_baseHelper_validate(t *testing.T) {
	type fields struct {
		Start           int64
		End             int64
		LastBlockHeight uint64
		Log             protocol.Logger
		Tx              *commonPb.Transaction
		store           protocol.BlockchainStore
		ruleCache       *ruleCache
		role            protocol.Role
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "异常流 invalid Start block height",
			fields: fields{
				Start: -2,
			},
			wantErr: true,
		},
		{
			name: "异常流 invalid End block height",
			fields: fields{
				End: -2,
			},
			wantErr: true,
		},
		{
			name: "异常流 invalid Start、End block height",
			fields: fields{
				End:   -2,
				Start: 1,
			},
			wantErr: true,
		},
		{
			name: "异常流 payload Start block height > last block height",
			fields: fields{
				End:             -1,
				Start:           1,
				LastBlockHeight: 0,
			},
			wantErr: true,
		},
		{
			name: "正常流 ",
			fields: fields{
				End:             -1,
				Start:           1,
				LastBlockHeight: 2,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SubscribeFilterManager{
				Start:           tt.fields.Start,
				End:             tt.fields.End,
				LastBlockHeight: tt.fields.LastBlockHeight,
				Log:             tt.fields.Log,
				Tx:              tt.fields.Tx,
				store:           tt.fields.store,
				ruleCache:       tt.fields.ruleCache,
				role:            tt.fields.role,
			}
			err := h.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
