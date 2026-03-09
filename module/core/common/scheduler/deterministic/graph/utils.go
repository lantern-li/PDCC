package graph

import (
	"sort"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
)

type MasterWriteSet map[string][]*commonPb.VersionedTxWrite // string：string(Write.Key)

// buildMasterWriteSet 将每笔交易的写集按序合并，生成写集多版本总表。
// 对于同一个key，masterWS中是升序的（按交易顺序）。
func buildMasterWriteSet(execInfos []txExecInfo) MasterWriteSet {
	masterWS := make(MasterWriteSet)
	for _, execInfo := range execInfos {
		// 对于每笔交易
		for _, vw := range execInfo.txWriteSetWithVersion {
			key := string(vw.Write.Key)
			masterWS[key] = append(masterWS[key], vw)
		}
	}
	return masterWS
}

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
