/*
   Created by guoxin in 2022/9/28 5:24 PM
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

func TestBlockHeaderSubscribeResult_GetResult(t *testing.T) {
	header := &commonPb.BlockHeader{BlockHeight: 1}
	type fields struct {
		store protocol.BlockchainStore
	}
	type args struct {
		height uint64
		in1    func(*commonPb.Block) []*commonPb.Transaction
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
				store.EXPECT().GetBlockHeaderByHeight(gomock.Any()).AnyTimes().Return(nil, errors.New("store error"))
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
				store.EXPECT().GetBlockHeaderByHeight(gomock.Any()).AnyTimes().Return(nil, nil)
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
				store.EXPECT().GetBlockHeaderByHeight(gomock.Any()).AnyTimes().Return(&commonPb.BlockHeader{BlockHeight: 1}, nil)
				return store
			}()},
			args: args{
				height: 0,
			},
			want: &commonPb.SubscribeResult{Data: func() []byte {
				data, err := proto.Marshal(header)
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
			b := BlockHeaderSubscribeResult{
				store: tt.fields.store,
			}
			got, err := b.GetResultByHeight(tt.args.height, tt.args.in1)
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

func TestBlockHeaderSubscribeResult_GetType(t *testing.T) {
	type fields struct {
		store protocol.BlockchainStore
	}
	tests := []struct {
		name   string
		fields fields
		want   Type
	}{
		{
			name:   "正常流",
			fields: fields{},
			want:   OnlyHeaderResultType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := BlockHeaderSubscribeResult{
				store: tt.fields.store,
			}
			if got := b.GetType(); got != tt.want {
				t.Errorf("GetType() = %v, want %v", got, tt.want)
			}
		})
	}
}
