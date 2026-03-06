package graph

import (
	"testing"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/stretchr/testify/assert"
)

func TestBuildMasterWriteSet_Empty(t *testing.T) {
	masterWS := buildMasterWriteSet([]txExecInfo{})
	assert.Empty(t, masterWS)
}

func TestBuildMasterWriteSet_SingleTx(t *testing.T) {
	execInfos := []txExecInfo{
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 0},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)

	assert.Len(t, masterWS, 2)
	assert.Len(t, masterWS["key1"], 1)
	assert.Len(t, masterWS["key2"], 1)
	assert.Equal(t, uint64(0), masterWS["key1"][0].Version)
}

func TestBuildMasterWriteSet_MultiTxSameKey(t *testing.T) {
	execInfos := []txExecInfo{
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 1},
			},
		},
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 2},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)

	assert.Len(t, masterWS, 1)
	assert.Len(t, masterWS["key1"], 3)
	// 验证按交易顺序升序排列
	assert.Equal(t, uint64(0), masterWS["key1"][0].Version)
	assert.Equal(t, uint64(1), masterWS["key1"][1].Version)
	assert.Equal(t, uint64(2), masterWS["key1"][2].Version)
}

func TestBuildMasterWriteSet_MultiTxDifferentKeys(t *testing.T) {
	execInfos := []txExecInfo{
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 1},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)

	assert.Len(t, masterWS, 2)
	assert.Len(t, masterWS["key1"], 1)
	assert.Len(t, masterWS["key2"], 1)
}

func TestBuildMasterWriteSet_TxWithNoWrites(t *testing.T) {
	execInfos := []txExecInfo{
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txWriteSetWithVersion: nil, // 无写集的交易
		},
		{
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 2},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)

	assert.Len(t, masterWS, 1)
	assert.Len(t, masterWS["key1"], 2)
	// 跳过无写集的交易后，顺序仍然正确
	assert.Equal(t, uint64(0), masterWS["key1"][0].Version)
	assert.Equal(t, uint64(2), masterWS["key1"][1].Version)
}
