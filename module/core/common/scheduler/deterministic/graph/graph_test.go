package graph

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== 环检测测试 ====================

// TestDetectCycle_NoCycle_DAG 测试无环的DAG
func TestDetectCycle_NoCycle_DAG(t *testing.T) {
	// 0 -> 1 -> 2 -> 3  (链式依赖，无环)
	graph := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
		},
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.False(t, hasCycle)
	assert.Nil(t, cycleNodes)
}

// TestDetectCycle_SimpleCycle 测试简单环: 0 -> 1 -> 2 -> 0
func TestDetectCycle_SimpleCycle(t *testing.T) {
	graph := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0},
		},
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.True(t, hasCycle)
	assert.NotEmpty(t, cycleNodes)
	t.Logf("cycle nodes: %v", cycleNodes)
}

// TestDetectCycle_TwoNodeCycle 测试两节点环: 0 <-> 1
func TestDetectCycle_TwoNodeCycle(t *testing.T) {
	graph := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {1},
			1: {0},
		},
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.True(t, hasCycle)
	assert.NotEmpty(t, cycleNodes)
	t.Logf("cycle nodes: %v", cycleNodes)
}

// TestDetectCycle_DisconnectedWithCycle 测试不连通图中一个连通分量有环
func TestDetectCycle_DisconnectedWithCycle(t *testing.T) {
	// 连通分量1: 0 -> 1 (无环)
	// 连通分量2: 2 -> 3 -> 4 -> 2 (有环)
	graph := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1},
			2: {3},
			3: {4},
			4: {2},
		},
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.True(t, hasCycle)
	assert.NotEmpty(t, cycleNodes)
	t.Logf("cycle nodes: %v", cycleNodes)
}

// TestDetectCycle_SingleNode 测试单节点无环
func TestDetectCycle_SingleNode(t *testing.T) {
	graph := &Graph{
		Nodes: []int{0},
		Edges: make(map[int][]int),
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.False(t, hasCycle)
	assert.Nil(t, cycleNodes)
}

// TestDetectCycle_EmptyGraph 测试空图
func TestDetectCycle_EmptyGraph(t *testing.T) {
	graph := &Graph{
		Nodes: []int{},
		Edges: make(map[int][]int),
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.False(t, hasCycle)
	assert.Nil(t, cycleNodes)
}

// TestDetectCycle_DiamondNoCycle 测试菱形图（无环）
func TestDetectCycle_DiamondNoCycle(t *testing.T) {
	//     0
	//    / \
	//   1   2
	//    \ /
	//     3
	graph := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 2},
			1: {3},
			2: {3},
		},
	}
	hasCycle, cycleNodes := graph.DetectCycle()
	assert.False(t, hasCycle)
	assert.Nil(t, cycleNodes)
}

// ==================== Copy 测试 ====================

// TestCopy_BasicDeepCopy 测试基本深拷贝：修改副本不影响原图
func TestCopy_BasicDeepCopy(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
		},
	}

	cp := g.Copy()

	// 验证副本内容与原图一致
	assert.Equal(t, g.Nodes, cp.Nodes)
	assert.Equal(t, g.Edges, cp.Edges)
	assert.Nil(t, cp.RemovedNodes)

	// 修改副本的 Nodes，不影响原图
	cp.Nodes[0] = 99
	assert.Equal(t, 0, g.Nodes[0], "修改副本 Nodes 不应影响原图")

	// 修改副本的 Edges，不影响原图
	cp.Edges[0] = append(cp.Edges[0], 99)
	assert.Equal(t, []int{1}, g.Edges[0], "修改副本 Edges 不应影响原图")

	// 给副本新增 Edge key，不影响原图
	cp.Edges[99] = []int{100}
	_, exists := g.Edges[99]
	assert.False(t, exists, "副本新增 Edge key 不应影响原图")
}

// TestCopy_WithRemovedNodes 测试包含 RemovedNodes 的深拷贝
func TestCopy_WithRemovedNodes(t *testing.T) {
	g := &Graph{
		Nodes:        []int{0, 1, 2, 3},
		Edges:        map[int][]int{0: {1}, 2: {3}},
		RemovedNodes: []int{4, 5},
	}

	cp := g.Copy()

	// 验证 RemovedNodes 被正确拷贝
	assert.Equal(t, g.RemovedNodes, cp.RemovedNodes)

	// 修改副本的 RemovedNodes，不影响原图
	cp.RemovedNodes[0] = 99
	assert.Equal(t, 4, g.RemovedNodes[0], "修改副本 RemovedNodes 不应影响原图")
}

// TestCopy_EmptyGraph 测试空图的深拷贝
func TestCopy_EmptyGraph(t *testing.T) {
	g := &Graph{
		Nodes: []int{},
		Edges: make(map[int][]int),
	}

	cp := g.Copy()

	assert.Equal(t, 0, len(cp.Nodes))
	assert.Equal(t, 0, len(cp.Edges))
	assert.Nil(t, cp.RemovedNodes)
}

// TestCopy_SingleNode 测试单节点无边图的深拷贝
func TestCopy_SingleNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{0},
		Edges: make(map[int][]int),
	}

	cp := g.Copy()

	assert.Equal(t, []int{0}, cp.Nodes)
	assert.Equal(t, 0, len(cp.Edges))
}

// TestCopy_NilRemovedNodes 测试 RemovedNodes 为 nil 时，副本也为 nil
func TestCopy_NilRemovedNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{0: {1}},
		// RemovedNodes 未设置，为 nil
	}

	cp := g.Copy()

	assert.Nil(t, cp.RemovedNodes)
}

// TestCopy_EmptyRemovedNodes 测试 RemovedNodes 为空切片（len=0）时，副本也为 nil
func TestCopy_EmptyRemovedNodes(t *testing.T) {
	g := &Graph{
		Nodes:        []int{0},
		Edges:        make(map[int][]int),
		RemovedNodes: []int{}, // 空切片
	}

	cp := g.Copy()

	// len(g.RemovedNodes) == 0，所以不会进入拷贝分支，副本应为 nil
	assert.Nil(t, cp.RemovedNodes)
}

// TestCopy_ComplexGraph 测试复杂图的深拷贝
func TestCopy_ComplexGraph(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1, 2},
			1: {3},
			2: {3, 4},
			4: {0},
		},
		RemovedNodes: []int{5, 6, 7},
	}

	cp := g.Copy()

	// 验证所有字段
	assert.Equal(t, g.Nodes, cp.Nodes)
	assert.Equal(t, g.Edges, cp.Edges)
	assert.Equal(t, g.RemovedNodes, cp.RemovedNodes)

	// 确保是不同的底层数组
	assert.NotSame(t, &g.Nodes[0], &cp.Nodes[0])
	assert.NotSame(t, &g.RemovedNodes[0], &cp.RemovedNodes[0])
	for k := range g.Edges {
		if len(g.Edges[k]) > 0 {
			assert.NotSame(t, &g.Edges[k][0], &cp.Edges[k][0])
		}
	}
}

// TestCopy_MultipleEdgesPerNode 测试一个节点有多条边的情况
func TestCopy_MultipleEdgesPerNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 2, 3},
		},
	}

	cp := g.Copy()

	assert.Equal(t, []int{1, 2, 3}, cp.Edges[0])

	// 修改副本的边列表
	cp.Edges[0][0] = 99
	assert.Equal(t, 1, g.Edges[0][0], "修改副本的边值不应影响原图")
}

// ==================== SubGraph 测试 ====================

// TestSubGraph_FullNodes 提取所有节点，应与原图等价
func TestSubGraph_FullNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0},
		},
	}

	sub := g.SubGraph([]int{0, 1, 2})

	assert.Equal(t, []int{0, 1, 2}, sub.Nodes)
	assert.Equal(t, []int{1}, sub.Edges[0])
	assert.Equal(t, []int{2}, sub.Edges[1])
	assert.Equal(t, []int{0}, sub.Edges[2])
}

// TestSubGraph_PartialNodes 提取部分节点，跨子图的边被过滤
func TestSubGraph_PartialNodes(t *testing.T) {
	// 0 -> 1 -> 2 -> 3
	// 提取 {1, 2}，只保留 1->2
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
		},
	}

	sub := g.SubGraph([]int{1, 2})

	assert.Equal(t, []int{1, 2}, sub.Nodes)
	assert.Equal(t, 1, len(sub.Edges))
	assert.Equal(t, []int{2}, sub.Edges[1])
}

// TestSubGraph_NoEdgesRetained 提取的节点之间无边
func TestSubGraph_NoEdgesRetained(t *testing.T) {
	// 0 -> 1, 2 -> 3
	// 提取 {0, 3}，没有 0->3 的边
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			2: {3},
		},
	}

	sub := g.SubGraph([]int{0, 3})

	assert.Equal(t, []int{0, 3}, sub.Nodes)
	assert.Equal(t, 0, len(sub.Edges))
}

// TestSubGraph_SingleNode 提取单个节点
func TestSubGraph_SingleNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
		},
	}

	sub := g.SubGraph([]int{1})

	assert.Equal(t, []int{1}, sub.Nodes)
	assert.Equal(t, 0, len(sub.Edges))
}

// TestSubGraph_EmptyNodes 提取空节点集
func TestSubGraph_EmptyNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
		},
	}

	sub := g.SubGraph([]int{})

	assert.Equal(t, 0, len(sub.Nodes))
	assert.Equal(t, 0, len(sub.Edges))
}

// TestSubGraph_CycleSubset 从含环图中提取环的子集
func TestSubGraph_CycleSubset(t *testing.T) {
	// 环: 0->1->2->0, 尾巴: 3->0
	// 提取 {0, 1, 2}，保留完整环
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0},
			3: {0},
		},
	}

	sub := g.SubGraph([]int{0, 1, 2})

	assert.Equal(t, []int{0, 1, 2}, sub.Nodes)
	assert.Equal(t, []int{1}, sub.Edges[0])
	assert.Equal(t, []int{2}, sub.Edges[1])
	assert.Equal(t, []int{0}, sub.Edges[2])
}

// TestSubGraph_MultipleEdgesFiltered 节点有多条出边，部分被过滤
func TestSubGraph_MultipleEdgesFiltered(t *testing.T) {
	// 0 -> {1, 2, 3}
	// 提取 {0, 1, 3}，只保留 0->1 和 0->3
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 2, 3},
		},
	}

	sub := g.SubGraph([]int{0, 1, 3})

	assert.Equal(t, []int{0, 1, 3}, sub.Nodes)
	assert.Equal(t, []int{1, 3}, sub.Edges[0])
}

// TestSubGraph_DoesNotModifyOriginal 提取子图不影响原图
func TestSubGraph_DoesNotModifyOriginal(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1, 2},
			1: {2},
		},
	}

	sub := g.SubGraph([]int{0, 1})

	// 修改子图
	sub.Nodes[0] = 99
	sub.Edges[0] = append(sub.Edges[0], 99)

	// 原图不受影响
	assert.Equal(t, []int{0, 1, 2}, g.Nodes)
	assert.Equal(t, []int{1, 2}, g.Edges[0])
}

// ==================== RemoveLeafNodes 测试 ====================

// TestRemoveLeafNodes_AllLeaves 所有节点都是叶子（无环），应全部删除
func TestRemoveLeafNodes_AllLeaves(t *testing.T) {
	// 0 -> 1 -> 2 (链式，入度=0的是0，出度=0的是2，迭代删除后全部清空)
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
		},
	}

	g.RemoveLeafNodes()

	assert.Equal(t, 0, len(g.Nodes), "链式无环图的所有节点应被删除")
	assert.Equal(t, 0, len(g.Edges), "所有边应被删除")
	fmt.Print(g.Nodes)
	fmt.Print(g.Edges)
}

// TestRemoveLeafNodes_SimpleRing 简单环，所有节点入度和出度均>0，不删除
func TestRemoveLeafNodes_SimpleRing(t *testing.T) {
	// 0 -> 1 -> 2 -> 0
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{0, 1, 2}, g.Nodes, "环上所有节点不应被删除")
}

// TestRemoveLeafNodes_RingWithTail 环带尾巴：0 -> 1 -> 2 -> 1 (环: 1<->2)，0是入度为0的叶子
func TestRemoveLeafNodes_RingWithTail(t *testing.T) {
	// 0 -> 1 -> 2 -> 1
	// 节点0入度为0，是叶子；删除0后，1和2形成环，不再删除
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {1},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{1, 2}, g.Nodes, "只有环上节点保留")
}

// TestRemoveLeafNodes_RingWithOutgoingLeaf 环节点有出边指向非环节点
func TestRemoveLeafNodes_RingWithOutgoingLeaf(t *testing.T) {
	// 环: 0 -> 1 -> 0
	// 出边: 1 -> 2 (2是出度为0的叶子)
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {0, 2},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{0, 1}, g.Nodes, "非环叶子节点2应被删除")
	// 验证边：1 -> 2 的边应被清除
	assert.Equal(t, []int{0}, g.Edges[1], "节点1指向2的边应被移除")
}

// TestRemoveLeafNodes_EmptyGraph 空图不报错
func TestRemoveLeafNodes_EmptyGraph(t *testing.T) {
	g := &Graph{
		Nodes: []int{},
		Edges: make(map[int][]int),
	}

	g.RemoveLeafNodes()

	assert.Equal(t, 0, len(g.Nodes))
}

// TestRemoveLeafNodes_SingleNode 单节点无边，入度=出度=0，应被删除
func TestRemoveLeafNodes_SingleNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{0},
		Edges: make(map[int][]int),
	}

	g.RemoveLeafNodes()

	assert.Equal(t, 0, len(g.Nodes))
}

// TestRemoveLeafNodes_DisconnectedNodes 多个孤立节点，全部删除
func TestRemoveLeafNodes_DisconnectedNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: make(map[int][]int),
	}

	g.RemoveLeafNodes()

	assert.Equal(t, 0, len(g.Nodes))
}

// TestRemoveLeafNodes_TwoNodeCycle 两节点环
func TestRemoveLeafNodes_TwoNodeCycle(t *testing.T) {
	// 0 <-> 1
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {1},
			1: {0},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{0, 1}, g.Nodes)
}

// TestRemoveLeafNodes_ComplexGraphWithCycleAndLeaves 复杂图：环加多层叶子
func TestRemoveLeafNodes_ComplexGraphWithCycleAndLeaves(t *testing.T) {
	// 图结构：
	//   5 -> 3 -> 4 -> 3  (环: 3 <-> 4)
	//   0 -> 3
	//   4 -> 6
	//   1 -> 2 (与环无关的链)
	//
	// 叶子节点迭代删除过程：
	//   第1轮：入度0 = {0, 1, 5}, 出度0 = {2, 6}  => 删除 {0, 1, 2, 5, 6}
	//   第2轮：剩余 {3, 4}，3<->4 形成环，入度和出度均>0，停止
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4, 5, 6},
		Edges: map[int][]int{
			0: {3},
			1: {2},
			3: {4},
			4: {3, 6},
			5: {3},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{3, 4}, g.Nodes, "只有环上节点 3 和 4 保留")
	// 验证边
	assert.Equal(t, []int{4}, g.Edges[3])
	assert.Equal(t, []int{3}, g.Edges[4])
}

// TestRemoveLeafNodes_MultipleDisconnectedCycles 多个独立的环
func TestRemoveLeafNodes_MultipleDisconnectedCycles(t *testing.T) {
	// 环1: 0 -> 1 -> 0
	// 环2: 2 -> 3 -> 4 -> 2
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1},
			1: {0},
			2: {3},
			3: {4},
			4: {2},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{0, 1, 2, 3, 4}, g.Nodes, "所有环节点都应保留")
}

// TestRemoveLeafNodes_IterativeRemoval 测试需要多轮迭代才能完全删除叶子的情况
func TestRemoveLeafNodes_IterativeRemoval(t *testing.T) {
	// 0 -> 1 -> 2 -> 3 -> 4 -> 5 -> 3 (环: 3->4->5->3)
	// 第1轮删除：0 (入度0)
	// 第2轮删除：1 (入度变为0)
	// 第3轮删除：2 (入度变为0)
	// 剩余：3->4->5->3 (环)
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4, 5},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
			3: {4},
			4: {5},
			5: {3},
		},
	}

	g.RemoveLeafNodes()

	sort.Ints(g.Nodes)
	assert.Equal(t, []int{3, 4, 5}, g.Nodes, "经过多轮迭代，只保留环 3->4->5->3")
}

// TestRemoveLeafNodes_DoesNotAffectRemovedNodesField 验证 RemoveLeafNodes 不会修改 RemovedNodes 字段
func TestRemoveLeafNodes_DoesNotAffectRemovedNodesField(t *testing.T) {
	g := &Graph{
		Nodes:        []int{0, 1, 2},
		Edges:        map[int][]int{0: {1}, 1: {2}},
		RemovedNodes: []int{99},
	}

	g.RemoveLeafNodes()

	// RemovedNodes 应保持原值（RemoveLeafNodes 不操作此字段）
	assert.Equal(t, []int{99}, g.RemovedNodes)
}

// TestRemoveLeafNodes_SelfLoop 自环节点（出度>0但入度也>0），不应被删除
func TestRemoveLeafNodes_SelfLoop(t *testing.T) {
	// 节点0有自环：0 -> 0
	// 节点1无边
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {0},
		},
	}

	g.RemoveLeafNodes()

	// 节点1: 入度=0且出度=0，被删除
	// 节点0: 自环，入度=1，出度=1，保留
	assert.Equal(t, []int{0}, g.Nodes, "自环节点应保留")
	assert.Equal(t, []int{0}, g.Edges[0])
}

// ==================== Copy + RemoveLeafNodes 集成测试 ====================

// TestCopy_ThenRemoveLeafNodes 验证 Copy 后对副本执行 RemoveLeafNodes 不影响原图
func TestCopy_ThenRemoveLeafNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
			3: {1}, // 环: 1->2->3->1, 叶子: 0
		},
	}

	// 保存原图状态
	origNodes := make([]int, len(g.Nodes))
	copy(origNodes, g.Nodes)
	origEdges := make(map[int][]int)
	for k, v := range g.Edges {
		tmp := make([]int, len(v))
		copy(tmp, v)
		origEdges[k] = tmp
	}

	// 拷贝并在副本上操作
	cp := g.Copy()
	cp.RemoveLeafNodes()

	// 验证副本上的 RemoveLeafNodes 结果
	sort.Ints(cp.Nodes)
	assert.Equal(t, []int{1, 2, 3}, cp.Nodes, "副本中只保留环节点")

	// 验证原图未被修改
	assert.Equal(t, origNodes, g.Nodes, "原图 Nodes 不应改变")
	assert.Equal(t, origEdges, g.Edges, "原图 Edges 不应改变")
}

// ==================== FindSCCs 测试 ====================

// normalizeSCCs 将 SCC 结果标准化：先对每个 SCC 内部排序，再按最小节点对所有 SCC 排序，方便断言比较。
func normalizeSCCs(sccs [][]int) [][]int {
	for _, scc := range sccs {
		sort.Ints(scc)
	}
	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})
	return sccs
}

// TestFindSCCs_EmptyGraph 空图，无 SCC
func TestFindSCCs_EmptyGraph(t *testing.T) {
	g := &Graph{
		Nodes: []int{},
		Edges: make(map[int][]int),
	}
	sccs := g.FindSCCs()
	assert.Equal(t, 0, len(sccs))
}

// TestFindSCCs_SingleNode 单节点无边，自身构成一个大小为 1 的 SCC
func TestFindSCCs_SingleNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{0},
		Edges: make(map[int][]int),
	}
	sccs := g.FindSCCs()
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0}, sccs[0])
}

// TestFindSCCs_IsolatedNodes 多个孤立节点，每个节点各自一个 SCC
func TestFindSCCs_IsolatedNodes(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: make(map[int][]int),
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 4, len(sccs))
	for i, scc := range sccs {
		assert.Equal(t, []int{i}, scc)
	}
}

// TestFindSCCs_SimpleChain 链式 DAG: 0→1→2→3，每个节点独立一个 SCC
func TestFindSCCs_SimpleChain(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 4, len(sccs))
	for i, scc := range sccs {
		assert.Equal(t, 1, len(scc))
		assert.Equal(t, i, scc[0])
	}
}

// TestFindSCCs_SimpleCycle 简单三节点环: 0→1→2→0，整体为一个 SCC
func TestFindSCCs_SimpleCycle(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0, 1, 2}, sccs[0])
}

// TestFindSCCs_TwoNodeCycle 两节点互指: 0↔1
func TestFindSCCs_TwoNodeCycle(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {1},
			1: {0},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0, 1}, sccs[0])
}

// TestFindSCCs_CycleWithTail 环带尾巴: 0→1→2→1，节点 0 单独一个 SCC，{1,2} 构成环 SCC
func TestFindSCCs_CycleWithTail(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {1},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 2, len(sccs))
	assert.Equal(t, []int{0}, sccs[0])
	assert.Equal(t, []int{1, 2}, sccs[1])
}

// TestFindSCCs_CycleWithOutgoing 环有出边指向外部: 0↔1, 1→2，节点 2 单独 SCC，{0,1} 环 SCC
func TestFindSCCs_CycleWithOutgoing(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1},
			1: {0, 2},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 2, len(sccs))
	assert.Equal(t, []int{0, 1}, sccs[0])
	assert.Equal(t, []int{2}, sccs[1])
}

// TestFindSCCs_TwoDisconnectedCycles 两个独立环
func TestFindSCCs_TwoDisconnectedCycles(t *testing.T) {
	// 环1: 0↔1
	// 环2: 2→3→4→2
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1},
			1: {0},
			2: {3},
			3: {4},
			4: {2},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 2, len(sccs))
	assert.Equal(t, []int{0, 1}, sccs[0])
	assert.Equal(t, []int{2, 3, 4}, sccs[1])
}

// TestFindSCCs_Diamond 菱形 DAG（无环），4 个大小为 1 的 SCC
func TestFindSCCs_Diamond(t *testing.T) {
	//     0
	//    / \
	//   1   2
	//    \ /
	//     3
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 2},
			1: {3},
			2: {3},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 4, len(sccs))
	for _, scc := range sccs {
		assert.Equal(t, 1, len(scc))
	}
}

// TestFindSCCs_SelfLoop 自环: 0→0，节点 0 自身构成 SCC
func TestFindSCCs_SelfLoop(t *testing.T) {
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {0},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 2, len(sccs))
	assert.Equal(t, []int{0}, sccs[0])
	assert.Equal(t, []int{1}, sccs[1])
}

// TestFindSCCs_ComplexMultipleSCCs 复杂图：多个 SCC 之间有连接
func TestFindSCCs_ComplexMultipleSCCs(t *testing.T) {
	// SCC-A: {0, 1, 2}  环 0→1→2→0
	// SCC-B: {3, 4}     环 3→4→3
	// SCC-C: {5}        孤立
	// SCC 之间的边: 2→3, 4→5
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4, 5},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0, 3},
			3: {4},
			4: {3, 5},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 3, len(sccs))
	assert.Equal(t, []int{0, 1, 2}, sccs[0])
	assert.Equal(t, []int{3, 4}, sccs[1])
	assert.Equal(t, []int{5}, sccs[2])
}

// TestFindSCCs_AllNodesInOneSCC 所有节点构成一个大 SCC（完全图）
func TestFindSCCs_AllNodesInOneSCC(t *testing.T) {
	// 0→1→2→3→0，同时 0→2, 1→3 使得所有节点强连通
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 2},
			1: {2, 3},
			2: {3},
			3: {0},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0, 1, 2, 3}, sccs[0])
}

// TestFindSCCs_LongChainWithCycleAtEnd 长链尾部带环
func TestFindSCCs_LongChainWithCycleAtEnd(t *testing.T) {
	// 0→1→2→3→4→5→3 (环: 3→4→5→3)
	// 前面 0,1,2 各自为独立 SCC
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4, 5},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
			3: {4},
			4: {5},
			5: {3},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 4, len(sccs))
	assert.Equal(t, []int{0}, sccs[0])
	assert.Equal(t, []int{1}, sccs[1])
	assert.Equal(t, []int{2}, sccs[2])
	assert.Equal(t, []int{3, 4, 5}, sccs[3])
}

// TestFindSCCs_TwoOverlappingCycles 两个共享节点的环形成一个更大的 SCC
func TestFindSCCs_TwoOverlappingCycles(t *testing.T) {
	// 环1: 0→1→2→0
	// 环2: 2→3→4→2
	// 共享节点 2，整体 {0,1,2,3,4} 构成一个 SCC
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {0, 3},
			3: {4},
			4: {2},
		},
	}
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0, 1, 2, 3, 4}, sccs[0])
}

// TestFindSCCs_SCCCount 验证 SCC 数量与节点总数的关系
func TestFindSCCs_SCCCount(t *testing.T) {
	// DAG 中 SCC 数量 == 节点数量
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1, 2},
			1: {3},
			2: {3, 4},
		},
	}
	sccs := g.FindSCCs()
	assert.Equal(t, 5, len(sccs), "DAG 中每个节点独立为一个 SCC")

	// 所有节点在一个环中，SCC 数量 == 1
	g2 := &Graph{
		Nodes: []int{0, 1, 2, 3, 4},
		Edges: map[int][]int{
			0: {1}, 1: {2}, 2: {3}, 3: {4}, 4: {0},
		},
	}
	sccs2 := g2.FindSCCs()
	assert.Equal(t, 1, len(sccs2), "完整环中所有节点为一个 SCC")
}

// ==================== selectNodeToRemove 测试 ====================

// TestSelectNodeToRemove_ByTotalDegree 总度数（入度+出度）最大的节点被选中
func TestSelectNodeToRemove_ByTotalDegree(t *testing.T) {
	// 0: 入度=0, 出度=1, 总度=1
	// 1: 入度=2, 出度=2, 总度=4  ← 最大
	// 2: 入度=1, 出度=1, 总度=2
	// 3: 入度=1, 出度=0, 总度=1
	g := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1},
			1: {2, 3},
			2: {1},
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 1, victim)
}

// TestSelectNodeToRemove_TieBreakByInDegree 总度数相同时，入度更大的优先
func TestSelectNodeToRemove_TieBreakByInDegree(t *testing.T) {
	// 0: 入度=2, 出度=1, 总度=3
	// 1: 入度=1, 出度=2, 总度=3
	// 总度相同，0 入度更大 → 选 0
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {1},
			1: {0, 0}, // 节点1出度=2，节点0入度=2
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 0, victim)
}

// TestSelectNodeToRemove_TieBreakByIndex 总度数和入度都相同时，序号最大的优先
func TestSelectNodeToRemove_TieBreakByIndex(t *testing.T) {
	// 对称环: 0↔1↔2↔0
	// 每个节点: 入度=2, 出度=2, 总度=4
	// 全部相同 → 选序号最大的节点 2
	g := &Graph{
		Nodes: []int{0, 1, 2},
		Edges: map[int][]int{
			0: {1, 2},
			1: {0, 2},
			2: {0, 1},
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 2, victim)
}

// TestSelectNodeToRemove_SingleNode 单节点直接返回
func TestSelectNodeToRemove_SingleNode(t *testing.T) {
	g := &Graph{
		Nodes: []int{5},
		Edges: make(map[int][]int),
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 5, victim)
}

// TestSelectNodeToRemove_TwoNodeCycle 两节点互指，度数完全对称，选序号大的
func TestSelectNodeToRemove_TwoNodeCycle(t *testing.T) {
	// 0↔1, 每个节点: 入度=1, 出度=1, 总度=2 → 选序号最大的 1
	g := &Graph{
		Nodes: []int{0, 1},
		Edges: map[int][]int{
			0: {1},
			1: {0},
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 1, victim)
}

// TestSelectNodeToRemove_HubNode 中心节点（hub）被多条边指向，应被优先选中
func TestSelectNodeToRemove_HubNode(t *testing.T) {
	// 3 -> 0, 4 -> 0, 5 -> 0, 0 -> 3
	// 0: 入度=3, 出度=1, 总度=4  ← 最大
	// 3: 入度=1, 出度=1, 总度=2
	// 4: 入度=0, 出度=1, 总度=1
	// 5: 入度=0, 出度=1, 总度=1
	g := &Graph{
		Nodes: []int{0, 3, 4, 5},
		Edges: map[int][]int{
			0: {3},
			3: {0},
			4: {0},
			5: {0},
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 0, victim)
}

// TestSelectNodeToRemove_NonContiguousNodes 节点编号不连续
func TestSelectNodeToRemove_NonContiguousNodes(t *testing.T) {
	// 节点: 10, 20, 30
	// 10↔20, 20↔30
	// 10: 入度=1, 出度=1, 总度=2
	// 20: 入度=2, 出度=2, 总度=4  ← 最大
	// 30: 入度=1, 出度=1, 总度=2
	g := &Graph{
		Nodes: []int{10, 20, 30},
		Edges: map[int][]int{
			10: {20},
			20: {10, 30},
			30: {20},
		},
	}

	victim := g.selectNodeToRemove()
	assert.Equal(t, 20, victim)
}

// ==================== RemoveLeafNodes + FindSCCs 集成测试 ====================

// TestRemoveLeafNodes_ThenFindSCCs 模拟实际破环流程：先 RemoveLeafNodes，再 FindSCCs，验证结果中所有 SCC 大小 >= 2
func TestRemoveLeafNodes_ThenFindSCCs(t *testing.T) {
	// 图结构：
	//   0→1→2→3→4→2 (环: 2→3→4→2)
	//   4→5          (5 是叶子)
	//   6→7→6        (独立环: 6↔7)
	//   8             (孤立节点)
	g := &Graph{
		Nodes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		Edges: map[int][]int{
			0: {1},
			1: {2},
			2: {3},
			3: {4},
			4: {2, 5},
			6: {7},
			7: {6},
		},
	}

	// 先剥离叶子节点
	g.RemoveLeafNodes()

	// 剩余节点应只有环上的: {2,3,4} 和 {6,7}
	sort.Ints(g.Nodes)
	assert.Equal(t, []int{2, 3, 4, 6, 7}, g.Nodes)

	// FindSCCs 应得到 2 个 SCC，且每个 SCC 大小 >= 2
	sccs := normalizeSCCs(g.FindSCCs())
	assert.Equal(t, 2, len(sccs))
	assert.Equal(t, []int{2, 3, 4}, sccs[0])
	assert.Equal(t, []int{6, 7}, sccs[1])

	for _, scc := range sccs {
		assert.GreaterOrEqual(t, len(scc), 2, "RemoveLeafNodes 后每个 SCC 大小应 >= 2")
	}
}

// TestCopy_RemoveLeafNodes_FindSCCs 完整模拟调度器中的破环流程
func TestCopy_RemoveLeafNodes_FindSCCs(t *testing.T) {
	// 原图: 0→1, 1→2, 2→0 (环), 0→3 (尾巴)
	original := &Graph{
		Nodes: []int{0, 1, 2, 3},
		Edges: map[int][]int{
			0: {1, 3},
			1: {2},
			2: {0},
		},
	}

	// 1. Copy
	graphCopy := original.Copy()
	// 2. RemoveLeafNodes
	graphCopy.RemoveLeafNodes()
	// 3. FindSCCs
	sccs := normalizeSCCs(graphCopy.FindSCCs())

	// 副本结果验证
	sort.Ints(graphCopy.Nodes)
	assert.Equal(t, []int{0, 1, 2}, graphCopy.Nodes, "叶子节点 3 应被删除")
	assert.Equal(t, 1, len(sccs))
	assert.Equal(t, []int{0, 1, 2}, sccs[0])

	// 原图不受影响
	assert.Equal(t, 4, len(original.Nodes))
	assert.Equal(t, []int{1, 3}, original.Edges[0])
}
