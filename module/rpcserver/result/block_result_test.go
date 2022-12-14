/*
   Created by guoxin in 2022/9/28 4:54 PM
*/
package result

import (
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"errors"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"reflect"
	"testing"
)

func TestBlockSubscribeResult_GetResult(t *testing.T) {
	transactions := []*commonPb.Transaction{{}, {}}
	type fields struct {
		store protocol.BlockchainStore
	}
	type args struct {
		height uint64
		filter func(*commonPb.Block) []*commonPb.Transaction
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonPb.SubscribeResult
		wantErr bool
	}{
		{
			name: "异常流 store error",
			fields: fields{store: func() protocol.BlockchainStore {
				store := mock.NewMockBlockchainStore(gomock.NewController(t))
				store.EXPECT().GetBlock(gomock.Any()).AnyTimes().Return(nil, errors.New("store error"))
				return store
			}()},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "正常流 未查询到区块",
			fields: fields{store: func() protocol.BlockchainStore {
				store := mock.NewMockBlockchainStore(gomock.NewController(t))
				store.EXPECT().GetBlock(gomock.Any()).AnyTimes().Return(nil, nil)
				return store
			}()},
			args:    args{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "正常流 查询到交易",
			fields: fields{store: func() protocol.BlockchainStore {
				store := mock.NewMockBlockchainStore(gomock.NewController(t))
				store.EXPECT().GetBlock(gomock.Any()).AnyTimes().Return(&commonPb.Block{Txs: transactions}, nil)
				return store
			}()},
			args: args{
				height: 0,
				filter: func(block *commonPb.Block) []*commonPb.Transaction {
					return transactions
				},
			},
			want: &commonPb.SubscribeResult{Data: func() []byte {
				data, err := proto.Marshal(&commonPb.BlockInfo{
					Block: &commonPb.Block{
						Header:         nil,
						Dag:            nil,
						AdditionalData: nil,
						Txs:            transactions,
					},
				})
				if err != nil {
					t.Error(err)
				}
				return data
			}()},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BlockSubscribeResult{
				store: tt.fields.store,
			}
			got, _, err := b.GetResultByHeight(tt.args.height, tt.args.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResult() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockSubscribeResult_GetType(t *testing.T) {
	type fields struct {
		store protocol.BlockchainStore
	}
	tests := []struct {
		name   string
		fields fields
		want   Type
	}{
		{
			name:   "",
			fields: fields{},
			want:   BlockResultType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BlockSubscribeResult{
				store: tt.fields.store,
			}
			if got := b.GetType(); got != tt.want {
				t.Errorf("GetType() = %v, want %v", got, tt.want)
			}
		})
	}
}
