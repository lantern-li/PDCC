package graph

import (
	"testing"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/stretchr/testify/assert"
)

// TestGraphBuild_NoConflict 测试无冲突场景：所有交易读写不同的key
func TestGraphBuild_NoConflict(t *testing.T) {
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key2")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 1},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key3")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key3")}, Version: 2},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：无冲突场景下，每个交易只依赖自己（已被过滤），所以边集为空
	assert.Equal(t, 3, len(graph.Nodes))
	assert.Equal(t, 0, len(graph.Edges))
}

// TestGraphBuild_SimpleConflict 测试简单冲突：tx1读取tx0写入的key
func TestGraphBuild_SimpleConflict(t *testing.T) {
	// tx0: 写 key1
	// tx1: 读 key1, 写 key2
	// 预期：tx1 -> tx0 (tx1依赖tx0)
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")}, // 读取 tx0 写入的 key1
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 1},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：tx1 -> tx0
	assert.Equal(t, 2, len(graph.Nodes))
	assert.Equal(t, 1, len(graph.Edges))
	assert.Contains(t, graph.Edges, 1)
	assert.Equal(t, []int{0}, graph.Edges[1])
}

// TestGraphBuild_MultipleReaders 测试多个读者：多个交易读取同一个写
func TestGraphBuild_MultipleReaders(t *testing.T) {
	// tx0: 写 key1
	// tx1: 读 key1
	// tx2: 读 key1
	// 预期：tx1 -> tx0, tx2 -> tx0
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：tx1 -> tx0, tx2 -> tx0
	assert.Equal(t, 3, len(graph.Nodes))
	assert.Equal(t, 2, len(graph.Edges))
	assert.Contains(t, graph.Edges, 1)
	assert.Contains(t, graph.Edges, 2)
	assert.Equal(t, []int{0}, graph.Edges[1])
	assert.Equal(t, []int{0}, graph.Edges[2])
}

// TestGraphBuild_MultipleWriters 测试多个写者：一个交易读取多个交易的写
func TestGraphBuild_MultipleWriters(t *testing.T) {
	// tx0: 写 key1
	// tx1: 写 key2
	// tx2: 读 key1, 读 key2
	// 预期：tx2 -> tx0, tx2 -> tx1
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 1},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
				{Key: []byte("key2")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：tx2 -> tx0, tx2 -> tx1
	assert.Equal(t, 3, len(graph.Nodes))
	assert.Equal(t, 1, len(graph.Edges))
	assert.Contains(t, graph.Edges, 2)
	assert.Equal(t, 2, len(graph.Edges[2]))
	assert.Contains(t, graph.Edges[2], 0)
	assert.Contains(t, graph.Edges[2], 1)
}

// TestGraphBuild_NoSelfLoop 测试自环过滤：交易读写同一个key不应产生自环
func TestGraphBuild_NoSelfLoop(t *testing.T) {
	// tx0: 读 key1, 写 key1
	// 预期：无边（自环被过滤）
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：无边（自环被过滤）
	assert.Equal(t, 1, len(graph.Nodes))
	assert.Equal(t, 0, len(graph.Edges))
}

// TestGraphBuild_DuplicateEdgeDedup 测试重复边去重：一笔交易读了另一笔交易写的多个key，只应记录一条边
func TestGraphBuild_DuplicateEdgeDedup(t *testing.T) {
	// tx0: 写 key_a, 写 key_b
	// tx1: 读 key_a, 读 key_b
	// 预期：tx1 -> tx0 只出现一次，不能出现 [0, 0]
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key_a")}, Version: 0},
				{Write: &commonPb.TxWrite{Key: []byte("key_b")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key_a")},
				{Key: []byte("key_b")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证：tx1 -> tx0 只出现一次
	assert.Equal(t, 2, len(graph.Nodes))
	assert.Equal(t, 1, len(graph.Edges))
	assert.Contains(t, graph.Edges, 1)
	assert.Equal(t, []int{0}, graph.Edges[1], "edge 1->0 should appear exactly once, got %v", graph.Edges[1])
}

// TestGraphBuild_DuplicateEdgeDedup_ThreeKeys 测试三个key重复的情况
func TestGraphBuild_DuplicateEdgeDedup_ThreeKeys(t *testing.T) {
	// tx0: 写 key_a, 写 key_b, 写 key_c
	// tx1: 读 key_a, 读 key_b, 读 key_c
	// 预期：tx1 -> tx0 只出现一次
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key_a")}, Version: 0},
				{Write: &commonPb.TxWrite{Key: []byte("key_b")}, Version: 0},
				{Write: &commonPb.TxWrite{Key: []byte("key_c")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key_a")},
				{Key: []byte("key_b")},
				{Key: []byte("key_c")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	assert.Equal(t, 2, len(graph.Nodes))
	assert.Equal(t, 1, len(graph.Edges))
	assert.Contains(t, graph.Edges, 1)
	assert.Equal(t, []int{0}, graph.Edges[1], "edge 1->0 should appear exactly once, got %v", graph.Edges[1])
}

// TestGraphBuild_DuplicateEdgeDedup_MutualConflict 测试双向冲突时的去重
func TestGraphBuild_DuplicateEdgeDedup_MutualConflict(t *testing.T) {
	// tx0: 读 key_a, 读 key_b, 写 key_c, 写 key_d
	// tx1: 读 key_c, 读 key_d, 写 key_a, 写 key_b
	// 预期：tx0 -> tx1 一条边，tx1 -> tx0 一条边（各只出现一次）
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key_a")},
				{Key: []byte("key_b")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key_c")}, Version: 0},
				{Write: &commonPb.TxWrite{Key: []byte("key_d")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key_c")},
				{Key: []byte("key_d")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key_a")}, Version: 1},
				{Write: &commonPb.TxWrite{Key: []byte("key_b")}, Version: 1},
			},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	assert.Equal(t, 2, len(graph.Nodes))
	assert.Equal(t, 2, len(graph.Edges))
	// tx0 -> tx1 只出现一次
	assert.Contains(t, graph.Edges, 0)
	assert.Equal(t, []int{1}, graph.Edges[0], "edge 0->1 should appear exactly once, got %v", graph.Edges[0])
	// tx1 -> tx0 只出现一次
	assert.Contains(t, graph.Edges, 1)
	assert.Equal(t, []int{0}, graph.Edges[1], "edge 1->0 should appear exactly once, got %v", graph.Edges[1])
}

// TestGraphBuild_ComplexScenario 测试复杂场景
func TestGraphBuild_ComplexScenario(t *testing.T) {
	// tx0: 写 key1
	// tx1: 读 key1, 写 key2
	// tx2: 读 key2, 写 key3
	// tx3: 读 key1, 读 key3
	// 预期：
	//   tx1 -> tx0
	//   tx2 -> tx1
	//   tx3 -> tx0, tx3 -> tx2
	execInfos := []txExecInfo{
		{
			txReadSet: []*commonPb.TxRead{},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key1")}, Version: 0},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key2")}, Version: 1},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key2")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{
				{Write: &commonPb.TxWrite{Key: []byte("key3")}, Version: 2},
			},
		},
		{
			txReadSet: []*commonPb.TxRead{
				{Key: []byte("key1")},
				{Key: []byte("key3")},
			},
			txWriteSetWithVersion: []*commonPb.VersionedTxWrite{},
		},
	}

	masterWS := buildMasterWriteSet(execInfos)
	graph := buildDependencyGraph(execInfos, masterWS)

	// 验证
	assert.Equal(t, 4, len(graph.Nodes))
	assert.Equal(t, 3, len(graph.Edges))

	// tx1 -> tx0
	assert.Contains(t, graph.Edges, 1)
	assert.Equal(t, []int{0}, graph.Edges[1])

	// tx2 -> tx1
	assert.Contains(t, graph.Edges, 2)
	assert.Equal(t, []int{1}, graph.Edges[2])

	// tx3 -> tx0, tx3 -> tx2
	assert.Contains(t, graph.Edges, 3)
	assert.Equal(t, 2, len(graph.Edges[3]))
	assert.Contains(t, graph.Edges[3], 0)
	assert.Contains(t, graph.Edges[3], 2)
}
