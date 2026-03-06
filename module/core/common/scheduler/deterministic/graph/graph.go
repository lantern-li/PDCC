package graph

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

// buildDependencyGraph 基于交易执行信息和写集总表构建读写依赖有向图。
// 节点为 execInfos 的索引，边 readerIdx -> writerIdx 表示读交易依赖写交易。
func buildDependencyGraph(execInfos []txExecInfo, masterWS MasterWriteSet) *Graph {
	graph := &Graph{
		Nodes: make([]int, len(execInfos)),
		Edges: make(map[int][]int),
	}

	for i := range execInfos {
		graph.Nodes[i] = i
	}

	for readerIdx, execInfo := range execInfos {
		for _, txRead := range execInfo.txReadSet {
			readKey := string(txRead.Key)
			if versionedWrites, exists := masterWS[readKey]; exists {
				for _, vw := range versionedWrites {
					writerIdx := int(vw.Version)
					// 避免自环：交易不能指向自己
					if writerIdx != readerIdx {
						graph.Edges[readerIdx] = append(graph.Edges[readerIdx], writerIdx)
					}
				}
			}
		}
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

	for _, scc := range sccs {
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
	return removedNodes
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
