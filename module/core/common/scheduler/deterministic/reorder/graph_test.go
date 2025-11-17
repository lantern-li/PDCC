package reorder

import (
	"strconv"
	"testing"
	"time"

	"chainmaker.org/chainmaker/common/v2/bitmap"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

const keyCtr = "conflictKey"

func TestCycleAlgorithm(t *testing.T) {
	graph := []*Vertex{
		{
			neighbors: []int{1},
			index:     -1,
			lowlink:   -1,
			onStack:   false,
			blocked:   false,
			bMap:      make(map[int]struct{}),
		},
		{
			neighbors: []int{2},
			index:     -1,
			lowlink:   -1,
			onStack:   false,
			blocked:   false,
			bMap:      make(map[int]struct{}),
		},
		{
			neighbors: []int{0},
			index:     -1,
			lowlink:   -1,
			onStack:   false,
			blocked:   false,
			bMap:      make(map[int]struct{}),
		},
		{
			neighbors: []int{},
			index:     -1,
			lowlink:   -1,
			onStack:   false,
			blocked:   false,
			bMap:      make(map[int]struct{}),
		},
	}
	tarjan := &TarjanAlgorithm{
		graph: graph,
		index: 0,
		s:     make([]int, 0, 4),
	}
	tarjan.Run()
	assert.Equal(t, graph[0].lowlink, graph[1].lowlink)
	assert.Equal(t, graph[0].lowlink, graph[2].lowlink)
	assert.NotEqual(t, graph[0].lowlink, graph[3].lowlink)

	johnson := &JohnsonAlgorithm{
		graph:  graph,
		index:  0,
		s:      make([]int, 0, 4),
		cycles: make([][]int, 0),
	}
	cycles, early := johnson.Run()
	assert.Equal(t, 1, len(cycles))
	assert.Equal(t, false, early)
}

func TestJohnsonAlgorithm(t *testing.T) {
	type fields struct {
		graph []*Vertex
	}
	var tests []struct {
		name   string
		fields fields
		want   int
	}
	//完全图测试（数据来自paper）
	//wants := []int{0, 0, 1, 5, 20, 84, 409, 2365, 16064, 125664}
	wants := []int{0, 0, 1, 5, 20, 84}
	for n := 2; n < len(wants); n++ {
		var graph []*Vertex
		for i := 0; i < n; i++ {
			var v Vertex
			v.neighbors = make([]int, 0, n-1)
			for j := 0; j < n; j++ {
				if j != i {
					v.neighbors = append(v.neighbors, j)
				}
			}
			graph = append(graph, &v)
		}
		tests = append(tests, struct {
			name   string
			fields fields
			want   int
		}{
			name:   strconv.Itoa(n),
			fields: fields{graph: graph},
			want:   wants[n],
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			johnson := newJohnsonAlgorithm(tt.fields.graph)
			cycles, early := johnson.Run()
			assert.Equal(t, tt.want, len(cycles))
			assert.Equal(t, false, early)
		})
	}
}

func Test_buildGraph(t *testing.T) {
	type args struct {
		readMap  map[string][]int
		writeMap map[string][]int
		size     int
	}
	tests := []struct {
		name string
		args args
		want []*Vertex
	}{
		{name: "1", args: args{
			readMap: map[string][]int{
				"pos1": {0},
				"pos2": {2, 4},
				"pos3": {2},
			},
			writeMap: map[string][]int{
				"pos1": {0, 1},
				"pos2": {3},
				"pos3": {3},
			},
			size: 5,
		}, want: []*Vertex{
			{neighbors: []int{}},
			{neighbors: []int{0}},
			{neighbors: []int{}},
			{neighbors: []int{2, 4}},
			{neighbors: []int{}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, buildGraph(tt.args.readMap, tt.args.writeMap, tt.args.size), "buildGraph2(%v, %v)", tt.args.readMap, tt.args.writeMap)
		})
	}
}
func Test_buildGraph2(t *testing.T) {
	type args struct {
		readMap  []*bitmap.Bitmap
		writeMap []*bitmap.Bitmap
	}
	tests := []struct {
		name string
		args args
		want []*Vertex
	}{
		{name: "1", args: args{
			readMap: []*bitmap.Bitmap{
				(&bitmap.Bitmap{}).Set(1), // bit position 1
				{},
				(&bitmap.Bitmap{}).Set(2).Set(3), // bit position 2,3
				{},
				(&bitmap.Bitmap{}).Set(2), // bit position 2
			},
			writeMap: []*bitmap.Bitmap{
				(&bitmap.Bitmap{}).Set(1), // bit position 1
				(&bitmap.Bitmap{}).Set(1), // bit position 1
				{},
				(&bitmap.Bitmap{}).Set(2).Set(3), // bit position 2,3
				{},
			},
		}, want: []*Vertex{
			{neighbors: []int{}},
			{neighbors: []int{0}},
			{neighbors: []int{}},
			{neighbors: []int{2, 4}},
			{neighbors: []int{}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, buildGraph2(tt.args.readMap, tt.args.writeMap), "buildGraph2(%v, %v)", tt.args.readMap, tt.args.writeMap)
		})
	}
}

func TestBuildDAG(t *testing.T) {
	ctrl := gomock.NewController(t)
	rwSets := []*commonPb.TxRWSet{
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key1")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key1")},
				{Key: []byte("key4")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key1")},
				{Key: []byte("key4")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key2")},
				{Key: []byte("key3")},
				{Key: []byte("key4")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key4")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key2")},
				{Key: []byte("key3")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key3")},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	var txResults []*TxResultIndex
	for i := 0; i < 5; i++ {
		simContext := mock.NewMockTxSimContext(ctrl)
		simContext.EXPECT().GetTxRWSet(gomock.Any()).Return(rwSets[i]).AnyTimes()
		txI := &TxResultIndex{
			Index:   i,
			Sim:     simContext,
			TxType:  0,
			Success: true,
		}
		txResults = append(txResults, txI)
	}
	readMap, writeMap := RWSet2Map(txResults)
	// graph should be: 0<->1; 1<->2; 2<-3; 4<-3
	graph, early := BuildGraphRemoveCycle(readMap, writeMap, len(txResults))
	// tx 1 appears in two cycles, should be first one to be removed
	assert.Equal(t, false, graph[0].Removed)
	assert.Equal(t, true, graph[1].Removed)
	assert.Equal(t, false, graph[2].Removed)
	assert.Equal(t, false, graph[3].Removed)
	assert.Equal(t, false, graph[4].Removed)
	assert.Equal(t, false, early)

	topo := NewTopologicalOrder(graph)
	reorder := topo.Run()
	// 2 and 4 should be before 3, no other constraint
	// the deterministic algorithm output this everytime
	assert.Equal(t, []int{0, 2, 4, 3}, reorder)
}

func TestBuildDAG2(t *testing.T) {
	ctrl := gomock.NewController(t)
	rwSets := []*commonPb.TxRWSet{
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key1")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key1")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key1")},
				{Key: []byte("key4")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key1")},
				{Key: []byte("key4")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key2")},
				{Key: []byte("key3")},
				{Key: []byte("key4")},
			},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key4")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{Key: []byte("key2")},
				{Key: []byte("key3")},
			},
		},
		{
			TxReads: []*commonPb.TxRead{
				{Key: []byte("key3")},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	var txResults []*TxResultIndex
	for i := 0; i < 5; i++ {
		simContext := mock.NewMockTxSimContext(ctrl)
		simContext.EXPECT().GetTxRWSet(gomock.Any()).Return(rwSets[i]).AnyTimes()
		txI := &TxResultIndex{
			Index:   i,
			Sim:     simContext,
			TxType:  0,
			Success: true,
		}
		txResults = append(txResults, txI)
	}
	readMap, writeMap := RWSet2Bitmap(txResults)
	// graph should be: 0<->1; 1<->2; 2<-3; 4<-3
	graph, early := BuildGraphRemoveCycle2(readMap, writeMap)
	// tx 1 appears in two cycles, should be first one to be removed
	assert.Equal(t, false, graph[0].Removed)
	assert.Equal(t, true, graph[1].Removed)
	assert.Equal(t, false, graph[2].Removed)
	assert.Equal(t, false, graph[3].Removed)
	assert.Equal(t, false, graph[4].Removed)
	assert.Equal(t, false, early)

	topo := NewTopologicalOrder(graph)
	reorder := topo.Run()
	// 2 and 4 should be before 3, no other constraint
	// the deterministic algorithm output this everytime
	assert.Equal(t, []int{0, 2, 4, 3}, reorder)
}

func TestBuildDAGTime(t *testing.T) {
	n := 5000
	//readMap := make([]*bitmap.Bitmap, 0)
	//writeMap := make([]*bitmap.Bitmap, 0)
	readMap := make(map[string][]int)
	writeMap := make(map[string][]int)
	for i := 0; i < n/2; i++ {
		//r := &bitmap.Bitmap{}
		//r.Set(i)
		//readMap = append(readMap, r)
		//w := &bitmap.Bitmap{}
		//w.Set(i)
		//writeMap = append(writeMap, w)
		str := strconv.Itoa(0)
		readMap[str] = append(readMap[str], i)
		for fuzz := 0; fuzz < 100; fuzz++ {
			str2 := strconv.Itoa(i + fuzz + 200000)
			readMap[str2] = []int{i}
		}
		writeMap[str] = append(writeMap[str], i)
	}
	for i := n / 2; i < n; i++ {
		str := strconv.Itoa(i)
		readMap[str] = append(readMap[str], i)
		for fuzz := 0; fuzz < 100; fuzz++ {
			str2 := strconv.Itoa(i + fuzz + 200000)
			readMap[str2] = []int{i}
		}
		writeMap[str] = append(writeMap[str], i)
	}
	startTime := time.Now()
	//graph := buildGraph2(readMap, writeMap)
	graph := buildGraph(readMap, writeMap, n)
	timeCostA := time.Since(startTime)
	johnson := newJohnsonAlgorithm(graph)
	cycles, earlyReturn := johnson.Run()
	//assert.Equal(t, false, earlyReturn)
	timeCostB := time.Since(startTime)
	if !earlyReturn {
		count, vertex2cycle := countAppearInCycle(cycles, len(readMap))
		timeCostC := time.Since(startTime)
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
		timeCostD := time.Since(startTime)
		topo := NewTopologicalOrder(graph)
		reorder := topo.Run()
		t.Logf("reorder: %v", reorder)
		timeCostE := time.Since(startTime)
		t.Logf("build graph cost %v, search graph cost %v, count cycle cost %v, remove vertex cost %v, topo cost %v", timeCostA, timeCostB-timeCostA, timeCostC-timeCostB, timeCostD-timeCostC, timeCostE-timeCostD)
	} else {
		t.Logf("early return, build graph cost %v, search graph cost %v", timeCostA, timeCostB-timeCostA)
	}
}

func TestBuildSCCTime(t *testing.T) {
	n := 5000
	readMap := make(map[string][]int)
	writeMap := make(map[string][]int)
	for i := 0; i < n/2; i++ {
		str := strconv.Itoa(0)
		readMap[str] = append(readMap[str], i)
		for fuzz := 0; fuzz < 100; fuzz++ {
			str2 := strconv.Itoa(i + fuzz + 200000)
			readMap[str2] = []int{i}
		}
		writeMap[str] = append(writeMap[str], i)
	}
	for i := n / 2; i < n; i++ {
		str := strconv.Itoa(i)
		readMap[str] = append(readMap[str], i)
		for fuzz := 0; fuzz < 100; fuzz++ {
			str2 := strconv.Itoa(i + fuzz + 200000)
			readMap[str2] = []int{i}
		}
		writeMap[str] = append(writeMap[str], i)
	}
	startTime := time.Now()
	graph := buildGraph(readMap, writeMap, n)
	timeCostA := time.Since(startTime)
	// clean variables
	initGraphVariables(graph)
	// run tarjan on subgraph >= johnson.index
	tarjan := &TarjanAlgorithm{
		graph:         graph,
		index:         0,
		s:             make([]int, 0, len(graph)),
		subgraphIndex: 0, //we do not use this, set to 0
	}
	tarjan.Run()
	for _, v := range graph {
		if v.index != v.lowlink {
			// this is NOT the first vertex in strongly connected component, remove it
			v.Removed = true
		}
	}
	//assert.Equal(t, false, earlyReturn)
	timeCostB := time.Since(startTime)
	topo := NewTopologicalOrder(graph)
	reorder := topo.Run()
	t.Logf("reorder: %v", reorder)
	timeCostC := time.Since(startTime)
	t.Logf("build graph cost %v, search graph cost %v, topo cost %v", timeCostA, timeCostB-timeCostA, timeCostC-timeCostB)
}

func TestEarlyReturnTime(t *testing.T) {
	n := 5000
	readMap := make(map[string][]int)
	writeMap := make(map[string][]int)

	for i := 0; i < n; i++ {
		readMap[keyCtr] = append(readMap[keyCtr], i)
		writeMap[keyCtr] = append(writeMap[keyCtr], i)
	}
	startTime := time.Now()
	graph := buildGraph(readMap, writeMap, n)
	timeCostA := time.Since(startTime)
	// clean variables
	initGraphVariables(graph)
	// run tarjan on subgraph >= johnson.index
	tarjan := &TarjanAlgorithm{
		graph:         graph,
		index:         0,
		s:             make([]int, 0, len(graph)),
		subgraphIndex: 0, //we do not use this, set to 0
	}
	tarjan.Run()
	for _, v := range graph {
		if v.index != v.lowlink {
			// this is NOT the first vertex in strongly connected component, remove it
			v.Removed = true
		}
	}
	//assert.Equal(t, false, earlyReturn)
	timeCostB := time.Since(startTime)
	topo := NewTopologicalOrder(graph)
	reorder := topo.Run()
	assert.Equal(t, 0, len(reorder))
	timeCostC := time.Since(startTime)
	graph = buildGraph(readMap, writeMap, n)
	timeCostD := time.Since(startTime)
	johnson := newJohnsonAlgorithm(graph)
	cycles, earlyReturn := johnson.Run()
	_ = cycles
	if !earlyReturn {
		count, vertex2cycle := countAppearInCycle(cycles, n)
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
	}
	timeCostE := time.Since(startTime)
	t.Logf("build graph cost %v, search graph cost %v, topo cost %v, build 2 cost %v, johnson cost %v", timeCostA, timeCostB-timeCostA, timeCostC-timeCostB, timeCostD-timeCostC, timeCostE-timeCostD)
}

func BenchmarkEarlyReturn(b *testing.B) {
	n := 5000
	readMap := make(map[string][]int)
	writeMap := make(map[string][]int)
	for i := 0; i < n; i++ {
		readMap[keyCtr] = append(readMap[keyCtr], i)
		writeMap[keyCtr] = append(writeMap[keyCtr], i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		graph, earlyReturn := BuildGraphRemoveSCC(readMap, writeMap, n)
		assert.Equal(b, true, earlyReturn)
		if !earlyReturn {
			topo := NewTopologicalOrder(graph)
			reorder := topo.Run()
			_ = reorder
		}
	}
}
