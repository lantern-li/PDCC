package graph

import "sort"

const (
	colorWhite = 0 // 未访问
	colorGray  = 1 // 正在访问（在当前DFS路径上）
	colorBlack = 2 // 已完成访问
)

type Graph struct {
	Nodes        []int         // 所有交易节点
	Edges        map[int][]int // 边: from -> []to (A依赖B，则A->B)
	RemovedNodes []int         // 因破环被移除的节点列表
}

// buildDependencyGraph 基于交易执行信息和写集总表构建读写依赖有向图（确定性构图）。
// 节点为 execInfos 的索引，边 readerIdx -> writerIdx 表示读交易依赖写交易。
func buildDependencyGraph(execInfos []txExecInfo, masterWS MasterWriteSet) *Graph {
	graph := &Graph{
		Nodes:        make([]int, len(execInfos)),
		Edges:        make(map[int][]int),
		RemovedNodes: make([]int, 0),
	}

	for i := range execInfos {
		graph.Nodes[i] = i
	}

	// 使用 map 对边进行去重：当一笔交易读了另一笔交易写的多个 key 时，只记录一条边 refactor
	edgeSet := make(map[int]map[int]struct{})

	for readerIdx, execInfo := range execInfos {
		for _, txRead := range execInfo.txReadSet {
			readKey := string(txRead.Key)
			if versionedWrites, exists := masterWS[readKey]; exists {
				for _, vw := range versionedWrites {
					writerIdx := int(vw.Version)
					// 避免自环：交易不能指向自己
					if writerIdx != readerIdx {
						if edgeSet[readerIdx] == nil {
							edgeSet[readerIdx] = make(map[int]struct{})
						}
						edgeSet[readerIdx][writerIdx] = struct{}{}
					}
				}
			}
		}
	}

	for readerIdx, writerSet := range edgeSet {
		writers := make([]int, 0, len(writerSet))
		for writerIdx := range writerSet {
			writers = append(writers, writerIdx)
		}
		sort.Ints(writers) // 升序排序
		graph.Edges[readerIdx] = writers
	}

	return graph
}

// Copy 深拷贝当前图，返回一个独立的副本。
func (g *Graph) Copy() *Graph {
	cp := &Graph{
		Nodes: make([]int, len(g.Nodes)),
		Edges: make(map[int][]int, len(g.Edges)),
	}
	copy(cp.Nodes, g.Nodes)
	for k, v := range g.Edges {
		edgesCopy := make([]int, len(v))
		copy(edgesCopy, v)
		cp.Edges[k] = edgesCopy
	}
	if len(g.RemovedNodes) > 0 {
		cp.RemovedNodes = make([]int, len(g.RemovedNodes))
		copy(cp.RemovedNodes, g.RemovedNodes)
	}
	return cp
}

// RemoveLeafNodes 循环删除所有入度为0或出度为0的节点及其关联边 todo：调研确认下这一步会减轻Tarjan算法的压力吗，值得吗？
func (g *Graph) RemoveLeafNodes() {
	for {
		// 计算每个节点的入度
		inDegree := make(map[int]int, len(g.Nodes)) // 每个节点入度表 map[int]int map[节点]入度数
		for _, node := range g.Nodes {
			// 先初始化每个节点的入度为0
			inDegree[node] = 0
		}
		for _, neighbors := range g.Edges {
			for _, to := range neighbors {
				inDegree[to]++
			}
		}

		// 找出入度为0或出度为0的节点
		var toRemove []int // 需要删除的节点
		for _, node := range g.Nodes {
			outDegree := len(g.Edges[node])
			if inDegree[node] == 0 || outDegree == 0 {
				toRemove = append(toRemove, node)
			}
		}

		if len(toRemove) == 0 {
			break
		}

		// 标记要删除的节点
		removeSet := make(map[int]bool, len(toRemove))
		for _, node := range toRemove {
			removeSet[node] = true
		}

		// 从 Nodes 中移除
		newNodes := make([]int, 0, len(g.Nodes)-len(toRemove)) // 剩余的节点
		for _, node := range g.Nodes {
			if !removeSet[node] {
				newNodes = append(newNodes, node)
			}
		}
		g.Nodes = newNodes

		// 删除被移除节点的出边
		for _, node := range toRemove {
			delete(g.Edges, node)
		}
		//并从其他节点的边中移除指向被删除节点的边
		for from, neighbors := range g.Edges {
			filtered := make([]int, 0, len(neighbors))
			for _, to := range neighbors {
				if !removeSet[to] {
					filtered = append(filtered, to)
				}
			}
			g.Edges[from] = filtered
		}
	}
}

// FindSCCs 使用 Tarjan 算法找出图中所有强连通分量（Strongly Connected Components）。
// 返回值: [][]int，每个元素是一个 SCC 的节点列表。
func (g *Graph) FindSCCs() [][]int {
	t := &tarjanState{
		graph:   g,
		index:   0,
		stack:   make([]int, 0),
		onStack: make(map[int]bool, len(g.Nodes)),
		nodeIdx: make(map[int]int, len(g.Nodes)),
		lowLink: make(map[int]int, len(g.Nodes)),
		visited: make(map[int]bool, len(g.Nodes)),
		sccs:    make([][]int, 0),
	}

	for _, node := range g.Nodes {
		if !t.visited[node] {
			t.strongConnect(node)
		}
	}
	return t.sccs
}

// tarjanState 保存 Tarjan 算法运行过程中的状态
type tarjanState struct {
	graph   *Graph
	index   int          // 全局递增索引
	stack   []int        // 当前 DFS 栈
	onStack map[int]bool // 节点是否在栈上
	nodeIdx map[int]int  // 每个节点的 DFS 序号
	lowLink map[int]int  // 每个节点能回溯到的最小 DFS 序号
	visited map[int]bool // 是否已访问
	sccs    [][]int      // 收集到的所有 SCC
}

// strongConnect 是 Tarjan 算法的核心递归过程
func (t *tarjanState) strongConnect(v int) {
	t.nodeIdx[v] = t.index
	t.lowLink[v] = t.index
	t.index++
	t.visited[v] = true

	// 将 v 压入栈
	t.stack = append(t.stack, v)
	t.onStack[v] = true

	// 遍历 v 的所有邻居
	for _, w := range t.graph.Edges[v] {
		if !t.visited[w] {
			// w 未访问，递归
			t.strongConnect(w)
			if t.lowLink[w] < t.lowLink[v] {
				t.lowLink[v] = t.lowLink[w]
			}
		} else if t.onStack[w] {
			// w 在栈上，说明 v-w 形成回边
			if t.nodeIdx[w] < t.lowLink[v] {
				t.lowLink[v] = t.nodeIdx[w]
			}
		}
	}

	// 如果 v 是 SCC 的根节点
	if t.lowLink[v] == t.nodeIdx[v] {
		var scc []int
		for {
			// 弹栈
			w := t.stack[len(t.stack)-1]
			t.stack = t.stack[:len(t.stack)-1]
			t.onStack[w] = false
			scc = append(scc, w)
			if w == v {
				break
			}
		}
		t.sccs = append(t.sccs, scc)
	}
}

// SubGraph 从当前图中提取由 nodes 指定的子图（仅保留 nodes 之间的边）。
func (g *Graph) SubGraph(nodes []int) *Graph {
	nodeSet := make(map[int]bool, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = true
	}

	sub := &Graph{
		Nodes: make([]int, len(nodes)),
		Edges: make(map[int][]int),
	}
	copy(sub.Nodes, nodes)

	for _, from := range nodes {
		if neighbors, ok := g.Edges[from]; ok {
			var filtered []int
			for _, to := range neighbors {
				if nodeSet[to] {
					filtered = append(filtered, to)
				}
			}
			if len(filtered) > 0 {
				sub.Edges[from] = filtered
			}
		}
	}
	return sub
}

// RemoveNode 从图中删除指定节点及其所有关联边。
func (g *Graph) RemoveNode(node int) {
	// 从 Nodes 中移除
	newNodes := make([]int, 0, len(g.Nodes)-1)
	for _, n := range g.Nodes {
		if n != node {
			newNodes = append(newNodes, n)
		}
	}
	g.Nodes = newNodes

	// 删除该节点的出边
	delete(g.Edges, node)

	// 从其他节点的边中移除指向该节点的边
	for from, neighbors := range g.Edges {
		filtered := make([]int, 0, len(neighbors))
		for _, to := range neighbors {
			if to != node {
				filtered = append(filtered, to)
			}
		}
		g.Edges[from] = filtered
	}
}

// selectNodeToRemove 按选点策略从图中选出一个要删除的节点。
// 策略：入度+出度最大 → 入度最大 → 序号最大。
func (g *Graph) selectNodeToRemove() int {
	// 计算入度
	inDegree := make(map[int]int, len(g.Nodes))
	for _, node := range g.Nodes {
		inDegree[node] = 0
	}
	for _, neighbors := range g.Edges {
		for _, to := range neighbors {
			inDegree[to]++
		}
	}

	bestNode := g.Nodes[0]
	bestIn := inDegree[bestNode]
	bestTotal := bestIn + len(g.Edges[bestNode])

	for _, node := range g.Nodes[1:] {
		in := inDegree[node]
		total := in + len(g.Edges[node])

		if total > bestTotal ||
			(total == bestTotal && in > bestIn) ||
			(total == bestTotal && in == bestIn && node > bestNode) {
			bestNode = node
			bestIn = in
			bestTotal = total
		}
	}
	return bestNode
}

// BreakCycles 对给定的所有 SCC 执行破环操作。
// 对每个 SCC：循环选点删除 → RemoveLeafNodes，直到 SCC 中剩余节点数量 <= 1。
// 返回所有被删除的节点列表。
func (g *Graph) BreakCycles(sccs [][]int) []int {
	var removedNodes []int

	for _, scc := range sccs { // todo：先串行吧，后面并行。
		if len(scc) < 2 { // todo：实际上不会有这种情况，但代码还是保留着吧。
			continue
		}

		// 从原图中提取该 SCC 的子图
		sub := g.SubGraph(scc)

		for len(sub.Nodes) > 1 {
			// 1. 按选点策略选一个节点删除
			victim := sub.selectNodeToRemove()
			sub.RemoveNode(victim)
			removedNodes = append(removedNodes, victim)

			// 2. 剥离叶子节点
			sub.RemoveLeafNodes()
		}
	}

	sort.Ints(removedNodes) // 升序排序
	return removedNodes
}

// MarkCommittable 基于 Sink Nodes 的反向传播标记，将所有节点标记为可提交（红色）或不可提交（灰色）。
//
// 算法（在破环后的 DAG 上执行）：
//
//	a. 所有出度为0的节点（Sink Nodes）可以提交 → 标记为红色-0
//	b. 直接依赖红色节点的未标记节点 → 标记为灰色
//	c. 仅依赖灰色节点的未标记节点 → 标记为红色
//	d. 重复 b、c 直到所有节点都被标记
//
// 边方向：from → to 表示 from 依赖 to（from 读了 to 的写集）。
// "直接依赖X"指的是节点有一条出边指向X颜色的节点。
//
// 返回值：committable 为可提交的节点列表（红色），uncommittable 为不可提交的节点列表（灰色）。
// todo：这里两点是很重要的，一点是确定性，还有一点是活性（即能反向传播整个图）
// todo：代码最后写完让AI评估下是否能反向传播完整个图
//func (g *Graph) MarkCommittable_Old() (committable, uncommittable []int) {
//	const (
//		unmarked = 0
//		red      = 1 // 可提交
//		gray     = 2 // 不可提交
//	)
//
//	mark := make(map[int]int, len(g.Nodes)) // 标记
//
//	// 构建反向邻接表：reverseEdges[to] = []from，即"谁依赖了 to"
//	reverseEdges := make(map[int][]int, len(g.Nodes))
//	for from, neighbors := range g.Edges {
//		for _, to := range neighbors {
//			reverseEdges[to] = append(reverseEdges[to], from)
//		}
//	}
//
//	// 第一轮：所有出度为0的节点标记为红色，加入队列
//	queue := make([]int, 0)
//	for _, node := range g.Nodes {
//		if len(g.Edges[node]) == 0 {
//			mark[node] = red
//			queue = append(queue, node)
//		}
//	}
//
//	// BFS 反向传播
//	for len(queue) > 0 {
//		// 取出当前批次
//		current := queue
//		queue = nil
//
//		// 收集所有被当前批次影响到的、还未标记的邻居（即"谁依赖了 current 中的节点"）
//		// 这些邻居根据 current 的颜色决定自己的颜色：
//		//   - current 是红色 → 邻居标灰
//		//   - current 是灰色 → 邻居如果所有依赖都已标记且没有红色依赖，则标红
//		affected := make(map[int]bool) // 受影响的未标记节点
//		for _, node := range current {
//			for _, from := range reverseEdges[node] {
//				if mark[from] == unmarked {
//					affected[from] = true // 该节点还未标记，确实是受影响了
//				}
//			}
//		}
//
//		// 对受影响的节点做出裁决
//		for node := range affected {
//			// 检查该节点的所有依赖（出边指向的节点）是否都已标记
//			allMarked := true
//			hasRedDep := false
//			for _, to := range g.Edges[node] {
//
//				if mark[to] == unmarked {
//					allMarked = false // 这个似乎要分灰层影响和红色影响不同进行分类讨论 todo：分类讨论重构代码吧。对于红色层：是只有有依赖就灰；对于灰色层，是仅依赖，才红。 然后让AI确认是是否能反向传播完整个图。
//					break             // 红色层反向传播不需要这样
//				}
//				if mark[to] == red {
//					hasRedDep = true
//				}
//			}
//			if !allMarked {
//				continue // 还有依赖未标记，等后续轮次处理
//			}
//
//			// 所有依赖都已标记：有红色依赖 → 灰色，仅灰色依赖 → 红色
//			if hasRedDep {
//				mark[node] = gray
//			} else {
//				mark[node] = red
//			}
//			queue = append(queue, node)
//		}
//	}
//
//	// 收集结果
//	for _, node := range g.Nodes {
//		if mark[node] == red {
//			committable = append(committable, node)
//		} else {
//			uncommittable = append(uncommittable, node)
//		}
//	}
//	return
//}

// MarkCommittable 基于 Sink Nodes 到 Source Nodes 的反向传播标记，将所有节点标记为可提交（红色）或不可提交（灰色）。
//
// 算法（在破环后的 DAG 上执行）：
//
//	a. 所有出度为0的节点（Sink Nodes）可以提交 → 标记为红色-0
//	b. 只要依赖红色节点的未标记节点 → 标记为灰色
//	c. 仅依赖灰色节点的未标记节点 → 标记为红色
//	d. 重复 b、c 直到所有节点都被标记
//
// 边方向：from → to 表示 from 依赖 to（from 读了 to 的写集）。
// "直接依赖X"指的是节点有一条出边指向X颜色的节点。
//
// 返回值：committable 为可提交的节点列表（红色），uncommittable 为不可提交的节点列表（灰色）。
// todo：这里两点是很重要的，一点是确定性，还有一点是活性（即能反向传播整个图）
// todo：代码最后写完让AI评估下是否能反向传播完整个图（只要是DAG就能保证） 待仔细证明
func (g *Graph) MarkCommittable() (committable, uncommittable []int) {
	const (
		unmarked = 0
		red      = 1 // 可提交
		gray     = 2 // 不可提交
	)
	markCount := 0 // 总共标记的节点数

	mark := make(map[int]int, len(g.Nodes)) // 标记

	// 构建反向邻接表：reverseEdges[to] = []from，即"谁依赖了 to"
	reverseEdges := make(map[int][]int, len(g.Nodes))
	for from, neighbors := range g.Edges {
		for _, to := range neighbors {
			reverseEdges[to] = append(reverseEdges[to], from)
		}
	}

	// 第一轮：所有出度为0的节点标记为红色，加入layer
	layer := make([]int, 0)
	for _, node := range g.Nodes {
		if len(g.Edges[node]) == 0 {
			mark[node] = red
			layer = append(layer, node)
		}
	}
	markCount = markCount + len(layer)

	isRedLayer := true
	for markCount < len(g.Nodes) { // 只要还有节点没被标记，就一直循环。todo：在确定能反向传播完整个图的前提上。

		//nextLayerMap := make(map[int]bool) // 受影响的未标记节点 todo:注意这里是map，不会重复
		nextLayer := make([]int, 0)

		if isRedLayer {
			// 处理红色层反向传播，只要依赖红色顶点就直接标记为灰色
			for _, node := range layer {
				for _, from := range reverseEdges[node] {
					if mark[from] == unmarked { // 这里有unmarked标记，所以不会重复
						mark[from] = gray
						nextLayer = append(nextLayer, from)
					}
				}
			}
		} else {
			// 处理灰色层反向传播, 如果有未标记的节点仅依赖灰色节点，就标记为红色
			for _, node := range layer {
				for _, from := range reverseEdges[node] {
					if mark[from] == unmarked {

						// todo：注意收集结论。未标记节点的所有依赖 ∈ {gray, unmarked}。否则它早就在上一轮 被标灰了
						// 判定from顶点是否仅仅指向灰色节点 todo
						onlygray := true
						for _, to := range g.Edges[from] {
							if mark[to] == unmarked {
								// from 这个节点先不处理
								onlygray = false
								break
							}
						}

						// from节点仅仅指向灰色节点，标记为红色
						// todo：这里不是BUG。没有 unmarked 等价于 所有依赖都是 gray。所以这里 不是 bug
						if onlygray {
							mark[from] = red
							nextLayer = append(nextLayer, from)
						}

					}
				}
			}
		}

		markCount = markCount + len(nextLayer) //总共被标记的节点
		layer = nextLayer

		isRedLayer = !isRedLayer // 状态翻转
	}

	// 收集结果 给出的是升序排列的结果
	for _, node := range g.Nodes {
		if mark[node] == red {
			committable = append(committable, node)
		} else {
			uncommittable = append(uncommittable, node)
		}
	}

	return
}

// DetectCycle 使用DFS三色标记法检测有向图中是否存在环。
// 返回 (是否有环, 环上的节点列表)。
func (g *Graph) DetectCycle() (bool, []int) {
	color := make(map[int]int, len(g.Nodes)) // 默认 colorWhite

	for _, node := range g.Nodes {
		if color[node] == colorWhite {
			if cycleNodes := g.dfsDetectCycle(node, color); len(cycleNodes) > 0 {
				return true, cycleNodes
			}
		}
	}
	return false, nil
}

// dfsDetectCycle 对 node 执行DFS，发现环时返回环上的节点列表。
func (g *Graph) dfsDetectCycle(node int, color map[int]int) []int {
	color[node] = colorGray

	for _, neighbor := range g.Edges[node] {
		if color[neighbor] == colorGray {
			// 发现环：neighbor 是当前DFS路径上的祖先节点
			return []int{neighbor, node}
		}
		if color[neighbor] == colorWhite {
			if cycleNodes := g.dfsDetectCycle(neighbor, color); len(cycleNodes) > 0 {
				// 如果环还没闭合（首节点还没再次出现在尾部），把当前节点追加进去
				if cycleNodes[0] != cycleNodes[len(cycleNodes)-1] {
					cycleNodes = append(cycleNodes, node)
				}
				return cycleNodes
			}
		}
	}

	color[node] = colorBlack
	return nil
}
