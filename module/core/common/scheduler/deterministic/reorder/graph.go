package reorder

import (
	"fmt"
	"sort"
	"strings"

	"chainmaker.org/chainmaker/common/v2/bitmap"
)

const (
	earlyReturnCycleCount = 100
	earlyRemoveEdgeCount  = 10
)

// nolint:revive
type Vertex struct {
	neighbors []int
	// variables for Tarjan algorithm
	index       int //remember to init to -1
	lowlink     int //remember to init to -1
	onStack     bool
	moreThanOne bool // belongs to strongly connected component >1
	// variables for Johnson algorithm
	blocked bool
	bMap    map[int]struct{}
	// variables for removing tx (tx will be processed in the next run)
	Removed bool
}

// TarjanAlgorithm Tarjan算法
type TarjanAlgorithm struct {
	graph         []*Vertex
	index         int
	s             []int // stack
	subgraphIndex int   // only consider vertex index >= this; used by Johnson algorithm
}

// not set Removed to false
func initGraphVariables(graph []*Vertex) {
	for _, v := range graph {
		v.index = -1
		v.lowlink = -1
		v.onStack = false
		v.moreThanOne = false
		v.blocked = false
		v.bMap = make(map[int]struct{})
	}
}

func (tarjan *TarjanAlgorithm) strongConnect(_v int) {
	if _v < tarjan.subgraphIndex {
		return
	}
	v := tarjan.graph[_v]
	if v.Removed {
		return
	}
	v.index = tarjan.index
	v.lowlink = tarjan.index
	tarjan.index++
	// push to stack
	tarjan.s = append(tarjan.s, _v)
	v.onStack = true
	//刚添加元素到栈后立即检查栈是否为空，这个条件永远不会为真，代码无效。
	// if len(tarjan.s) == 0 {
	// 	return
	// }

	for _, _w := range v.neighbors {
		if _w < tarjan.subgraphIndex {
			continue
		}
		w := tarjan.graph[_w]
		if w.index == -1 {
			tarjan.strongConnect(_w)
			if w.lowlink < v.lowlink {
				v.lowlink = w.lowlink
			}
		} else if w.onStack {
			if w.index < v.lowlink {
				v.lowlink = w.index
			}
		}
	}

	if v.lowlink == v.index {
		moreThanOne := false
		for {
			_w := tarjan.s[len(tarjan.s)-1]
			tarjan.s = tarjan.s[0 : len(tarjan.s)-1]
			tarjan.graph[_w].onStack = false
			if _w == _v {
				break
			}
			moreThanOne = true
			tarjan.graph[_w].moreThanOne = true
		}
		v.moreThanOne = moreThanOne
	}
}

// nolint:revive
func (tarjan *TarjanAlgorithm) Run() {
	for _v := 0; _v < len(tarjan.graph); _v++ {
		if tarjan.graph[_v].index == -1 {
			tarjan.strongConnect(_v)
		}
	}
}

// JohnsonAlgorithm Johnson算法
type JohnsonAlgorithm struct {
	graph  []*Vertex
	index  int     // the start of a search
	s      []int   // stack
	cycles [][]int // 找到的所有环
}

func newJohnsonAlgorithm(graph []*Vertex) *JohnsonAlgorithm {
	return &JohnsonAlgorithm{
		graph:  graph,
		index:  0,
		s:      make([]int, 0, len(graph)),
		cycles: make([][]int, 0),
	}
}

// return shouldUnblock(refer to Johnson algorithm) and earlyReturn(too many cycle)
func (johnson *JohnsonAlgorithm) circuit(_v int) (bool, bool) {
	f := false
	v := johnson.graph[_v]
	if v.Removed {
		return f, false
	}
	// 步骤1: 当前节点入栈并标记为 blocked
	johnson.s = append(johnson.s, _v)
	v.blocked = true
	// 步骤2: 遍历所有邻居
	for _, _w := range v.neighbors {
		w := johnson.graph[_w]
		// 只处理同一个 SCC 内的边
		// skip if v, w are not in same strong connected component
		if w.lowlink != v.lowlink {
			continue
		}
		if _w == johnson.index {
			// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
			// 关键：找到环！（回到起点）
			// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
			// add the cycle to results
			cycle := make([]int, len(johnson.s))
			copy(cycle, johnson.s)
			johnson.cycles = append(johnson.cycles, cycle)
			f = true
			// 提前返回检查：环太多（> 100）
			if len(johnson.cycles) > earlyReturnCycleCount {
				return f, true
			}
		} else if !w.blocked {
			// 递归搜索邻居
			wj, earlyReturn := johnson.circuit(_w)
			if wj {
				f = true
			}
			if earlyReturn {
				return f, true
			}
		}
	}
	// 步骤3: 回溯处理
	if f {
		johnson.unblock(_v) // 找到环，解除阻塞
	} else {
		// 没找到环，添加到邻居的 bMap
		for _, _w := range v.neighbors {
			w := johnson.graph[_w]
			// skip if v, w are not in same strong connected component
			if w.lowlink != v.lowlink {
				continue
			}
			if _, ok := w.bMap[_v]; !ok {
				w.bMap[_v] = struct{}{}
			}
		}
	}
	// 步骤4: 出栈
	johnson.s = johnson.s[0 : len(johnson.s)-1]
	return f, false
}

func (johnson *JohnsonAlgorithm) unblock(_u int) {
	u := johnson.graph[_u]
	u.blocked = false
	for _w := range u.bMap {
		delete(u.bMap, _w)
		if johnson.graph[_w].blocked {
			johnson.unblock(_w)
		}
	}
}

// Run return cycles, false; if too many cycles, return nil, true
func (johnson *JohnsonAlgorithm) Run() ([][]int, bool) {
	for {
		// clean variables
		// 步骤1: 清理变量
		initGraphVariables(johnson.graph)
		// run tarjan on subgraph >= johnson.index

		// 步骤2: 在子图上运行 Tarjan 算法找 SCC
		tarjan := &TarjanAlgorithm{
			graph:         johnson.graph,
			index:         0,
			s:             make([]int, 0, len(johnson.graph)),
			subgraphIndex: johnson.index,
		}
		tarjan.Run()
		// update the index to be the least number index in any strong connected component
		// 步骤3: 找到第一个在 SCC 中的节点作为起点
		for johnson.index < len(johnson.graph) {
			v := johnson.graph[johnson.index]
			// if we searched this vertex, and it is in component >1
			if v.index != -1 && v.moreThanOne {
				break
			}
			johnson.index++
		}
		// 步骤4: 如果到达最后一个节点，结束
		// if we find the last vertex or larger, it cannot have loop; end the algorithm
		if johnson.index >= len(johnson.graph)-1 {
			break
		}
		// 步骤5: 从当前节点开始搜索环
		_, earlyReturn := johnson.circuit(johnson.index)
		if earlyReturn {
			return johnson.cycles, true
		}
		johnson.index++
	}
	return johnson.cycles, false
}

// TopologicalOrder represents a data structure used to calculate the topological order
// of vertices in a directed acyclic graph (DAG)
type TopologicalOrder struct {
	graph []*Vertex
	s     []int
}

// NewTopologicalOrder creates a new instance of the TopologicalOrder with the given graph.
func NewTopologicalOrder(graph []*Vertex) *TopologicalOrder {
	return &TopologicalOrder{graph: graph, s: make([]int, 0, len(graph))}
}

func (t *TopologicalOrder) dfs(_v int) {
	v := t.graph[_v]
	if v.Removed {
		return
	}
	v.onStack = true
	for _, _w := range v.neighbors {
		w := t.graph[_w]
		if !w.onStack {
			t.dfs(_w)
		}
	}
	t.s = append(t.s, _v)
}

// nolint:revive
func (t *TopologicalOrder) Run() []int {
	for _v := 0; _v < len(t.graph); _v++ {
		if !t.graph[_v].onStack {
			t.dfs(_v)
		}
	}
	return t.s
}

// BuildGraphRemoveCycle build graph and remove all cycles, return a dag (removed is set to true for some vertices)
// if too many cycles, return nil, true
func BuildGraphRemoveCycle(readMap, writeMap map[string][]int, txBatchSize int) ([]*Vertex, bool) {
	// STEP 1: 构建依赖图
	graph := buildGraph(readMap, writeMap, txBatchSize)
	// STEP 2: 运行 Johnson 算法找所有环
	johnson := newJohnsonAlgorithm(graph)
	cycles, earlyReturn := johnson.Run()
	if earlyReturn {
		// 环太多，提前返回，全部交易留待串行处理
		// early return to avoid wasting time
		return nil, true
	}
	//STEP 3：统计顶点在环中出现的次数
	count, vertex2cycle := countAppearInCycle(cycles, txBatchSize)
	// prepare graph variables for the final DAG
	initGraphVariables(graph)
	// remove cycles based on sorted count, a heuristic approach
	removedCycles := make(map[int]struct{})
	for _, cI := range count { // count 已按出现次数降序排序
		// if all cycles are removed, we get DAG
		if len(removedCycles) == len(cycles) {
			break // 所有环都被打破了
		}
		graph[cI.i].Removed = true // 移除出现次数最多的节点
		for _, cycleIndex := range vertex2cycle[cI.i] {
			removedCycles[cycleIndex] = struct{}{} // 标记该环已被打破
		}
	}
	return graph, false
}

// BuildGraphRemoveCycle2 build graph and remove all cycles, return a dag (removed is set to true for some vertices)
// if too many cycles, return nil, true
// deprecated
func BuildGraphRemoveCycle2(readMap, writeMap []*bitmap.Bitmap) ([]*Vertex, bool) {
	graph := buildGraph2(readMap, writeMap)
	johnson := newJohnsonAlgorithm(graph)
	cycles, earlyReturn := johnson.Run()
	if earlyReturn {
		// early return to avoid wasting time
		return nil, true
	}
	count, vertex2cycle := countAppearInCycle(cycles, len(readMap))
	// prepare graph variables for the final DAG
	initGraphVariables(graph)
	// remove cycles based on sorted count, a heuristic approach
	removedCycles := make(map[int]struct{})
	for _, cI := range count {
		// if all cycles are removed, we get DAG
		if len(removedCycles) == len(cycles) {
			break
		}
		graph[cI.i].Removed = true
		for _, cycleIndex := range vertex2cycle[cI.i] {
			removedCycles[cycleIndex] = struct{}{}
		}
	}
	return graph, false
}

// build graph for tx read-write conflict (tx_read <- tx_write)

/*
输入参数：
- readMap: map[string][]int - key → [读该key的交易索引列表]
- writeMap: map[string][]int - key → [写该key的交易索引列表]
- txBatchSize: int - 交易总数

输出：
- []*Vertex - 有向图的邻接表表示

目标：
构建 WAR（Write-After-Read）依赖图：如果 TX_i 写了某个 key，TX_j 读了这个 key，则添加边 TX_i
→ TX_j
*/
func buildGraph(readMap, writeMap map[string][]int, txBatchSize int) []*Vertex {
	graph := make([]*Vertex, txBatchSize)           // 最终结果图
	_graph := make([]map[int]struct{}, txBatchSize) // 临时邻接表（用于去重）
	for i := 0; i < txBatchSize; i++ {
		_graph[i] = make(map[int]struct{}) // 每个顶点的邻居集合（去重）
	}
	earlyRemove := make(map[int]struct{}) // 高冲突节点标记
	for key, writeList := range writeMap {
		if readList, ok := readMap[key]; ok {
			for _, src := range writeList {
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// 检查1：该写的交易是否已被标记移除
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				if _, ok := earlyRemove[src]; ok {
					continue
				}
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// 检查2：读的交易数量是否过多（入度限制）
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				if len(readList) > earlyRemoveEdgeCount {
					// skip a node with too many neighbors
					earlyRemove[src] = struct{}{}
					continue
				}
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// 核心：添加交易依赖边 (写 → 读)
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				for _, dst := range readList {
					// ignore self edge
					if dst != src {
						// use a map to avoid repeat edge
						_graph[src][dst] = struct{}{}
					}
				}
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// 检查3：出度是否过多（出度限制）
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				if len(_graph[src]) > earlyRemoveEdgeCount {
					// skip a node with too many neighbors
					earlyRemove[src] = struct{}{}
					//break
				}
			}
		}
	}
	// 如果：TX_i 写 key_a  AND  TX_j 读 key_a  AND  i ≠ j
	// 则：添加边  TX_i → TX_j  (写必须在读之前执行)
	for src, dstList := range _graph {
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		// 情况1：节点被标记为移除
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		if _, ok := earlyRemove[src]; ok {
			// skip a node with too many neighbors
			graph[src] = &Vertex{neighbors: make([]int, 0), Removed: true}
			continue
		}
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		// 情况2：正常节点，构造邻接表
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		v := &Vertex{neighbors: make([]int, 0, len(dstList))}
		for dst := range dstList {
			v.neighbors = append(v.neighbors, dst)
		}
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		// 关键：排序保证确定性（不同节点执行结果一致）
		// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
		// sort neighbors to ensure deterministic result
		sort.SliceStable(v.neighbors, func(i, j int) bool {
			return v.neighbors[i] < v.neighbors[j]
		})
		graph[src] = v
	}
	return graph
}

// build graph for tx read-write conflict (tx_read <- tx_write)
// deprecated
func buildGraph2(readMap, writeMap []*bitmap.Bitmap) []*Vertex {
	graph := make([]*Vertex, len(readMap))
	for src, _writeMap := range writeMap {
		v := &Vertex{neighbors: make([]int, 0)}
		for dst, _readMap := range readMap {
			// ignore self edge
			if dst != src {
				if _readMap.InterExist(_writeMap) {
					v.neighbors = append(v.neighbors, dst)
				}
			}
		}
		graph[src] = v
	}
	return graph
}

func countAppearInCycle(cycles [][]int, txBatchSize int) ([]cycleCountIndex, [][]int) {
	count := make([]cycleCountIndex, txBatchSize)
	vertex2cycle := make([][]int, txBatchSize)
	for i := 0; i < txBatchSize; i++ {
		count[i].i = i
		//vertex2cycle[i] = make([]int, 0)
	}
	for cycleIndex, cycle := range cycles {
		for _, v := range cycle {
			count[v].count++
			vertex2cycle[v] = append(vertex2cycle[v], cycleIndex)
		}
	}
	// sort the slice based on count
	// note: use slice stable to get deterministic result
	sort.SliceStable(count, func(i, j int) bool {
		return count[i].count > count[j].count
	})
	return count, vertex2cycle
}

// PrintGraph return string of this graph
func PrintGraph(graph []*Vertex) string {
	var builder strings.Builder
	for i, v := range graph {
		builder.WriteString(fmt.Sprintf("%d: %v\n", i, v.neighbors))
	}
	return builder.String()
}

// BuildGraphRemoveSCC build graph and remove all strongly component
// (only choose one vertex in a strongly connected component),
// return a dag (removed is set to true for some vertices)
func BuildGraphRemoveSCC(readMap, writeMap map[string][]int, txBatchSize int) (graph []*Vertex, earlyReturn bool) {
	// STEP 1: 构建依赖图
	graph = buildGraph(readMap, writeMap, txBatchSize)
	// clean variables
	initGraphVariables(graph)
	// run tarjan
	// STEP 2: 运行Tarjan算法
	tarjan := &TarjanAlgorithm{
		graph:         graph,
		index:         0,
		s:             make([]int, 0, len(graph)),
		subgraphIndex: 0, //we do not use this, set to 0
	}
	tarjan.Run()
	// STEP 3: 移除SCC中的非根节点
	removeCount := 0
	for _, v := range graph {
		if v.Removed {
			removeCount++
			continue
		}
		if v.index != v.lowlink {
			// this is NOT the first vertex in strongly connected component, remove it
			v.Removed = true
			removeCount++
		}
	}
	// STEP 4: 检查是否所有交易都被移除
	if removeCount == txBatchSize {
		earlyReturn = true
	}
	return
}
