package aria

import (
	"sync/atomic"
	"testing"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
)

// helper：快速构造 txExecInfo
func makeExecInfo(index int, writes ...[2]string) txExecInfo {
	var txWrites []*commonPb.TxWrite
	for _, w := range writes {
		txWrites = append(txWrites, &commonPb.TxWrite{
			ContractName: w[0],
			Key:          []byte(w[1]),
		})
	}
	return txExecInfo{
		index:      index,
		txWriteSet: txWrites,
	}
}

// helper：构造带读集和写集的 txExecInfo
func makeExecInfoRW(index int, reads [][2]string, writes [][2]string) txExecInfo {
	var txReads []*commonPb.TxRead
	for _, r := range reads {
		txReads = append(txReads, &commonPb.TxRead{
			ContractName: r[0],
			Key:          []byte(r[1]),
		})
	}
	var txWrites []*commonPb.TxWrite
	for _, w := range writes {
		txWrites = append(txWrites, &commonPb.TxWrite{
			ContractName: w[0],
			Key:          []byte(w[1]),
		})
	}
	return txExecInfo{
		index:      index,
		txReadSet:  txReads,
		txWriteSet: txWrites,
	}
}

func TestConstructKey(t *testing.T) {
	got := constructKey("contract1", []byte("keyA"))
	want := "contract1keyA"
	if got != want {
		t.Errorf("constructKey = %q, want %q", got, want)
	}
}

// 单个交易、单个 key，不冲突
func TestReserveWrite_SingleTx(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
	}

	table, aborted := reserveWrite(infos)

	// tx0 不应被 abort
	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}

	// reserveTable 应持有 tx0
	val, ok := table.Load(constructKey("c", []byte("k1")))
	if !ok {
		t.Fatal("reservation for c+k1 not found")
	}
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("reservation txIndex = %d, want 0", res.txIndex)
	}
}

// 无冲突：两个交易写不同 key
func TestReserveWrite_NoConflict(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
		makeExecInfo(1, [2]string{"c", "k2"}),
	}

	_, aborted := reserveWrite(infos)

	for i := 0; i < 2; i++ {
		if aborted[i].Load() {
			t.Errorf("tx%d should not be aborted", i)
		}
	}
}

// 两个交易写同一个 key：较大 TID 应被 abort，较小 TID 持有预留
func TestReserveWrite_ConflictSmallerWins(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
		makeExecInfo(1, [2]string{"c", "k1"}),
	}

	table, aborted := reserveWrite(infos)

	// tx0 不被 abort
	if aborted[0].Load() {
		t.Error("tx0 (smaller TID) should not be aborted")
	}
	// tx1 应被 abort
	if !aborted[1].Load() {
		t.Error("tx1 (larger TID) should be aborted")
	}

	// 预留应属于 tx0
	val, _ := table.Load(constructKey("c", []byte("k1")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("reservation holder = %d, want 0", res.txIndex)
	}
}

// 三个交易竞争同一个 key：最小 TID 获胜
func TestReserveWrite_ThreeWayConflict(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
		makeExecInfo(1, [2]string{"c", "k1"}),
		makeExecInfo(2, [2]string{"c", "k1"}),
	}

	table, aborted := reserveWrite(infos)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted")
	}
	if !aborted[2].Load() {
		t.Error("tx2 should be aborted")
	}

	val, _ := table.Load(constructKey("c", []byte("k1")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("reservation holder = %d, want 0", res.txIndex)
	}
}

// 交易写多个 key，即使部分 key 预留失败也必须继续预留剩余 key
func TestReserveWrite_PartialConflict_ContinuesReserving(t *testing.T) {
	// tx0 写 k1
	// tx1 写 k1 和 k2
	// tx1 在 k1 上预留失败（tx0 更小），但应在 k2 上成功预留
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
		makeExecInfo(1, [2]string{"c", "k1"}, [2]string{"c", "k2"}),
	}

	table, aborted := reserveWrite(infos)

	// tx1 应被 abort（因为 k1 预留失败）
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted due to k1 conflict")
	}

	// tx1 仍然应持有 k2 的预留
	val, ok := table.Load(constructKey("c", []byte("k2")))
	if !ok {
		t.Fatal("reservation for k2 not found — tx1 should have continued reserving")
	}
	res := val.(*reservation)
	if res.txIndex != 1 {
		t.Errorf("k2 reservation holder = %d, want 1", res.txIndex)
	}
}

// 较大 TID 先预留，之后被较小 TID 覆盖
func TestReserveWrite_LargerReservedFirst_ThenOverridden(t *testing.T) {
	// 串行模拟：tx2 先预留 k1，tx0 后覆盖
	// 用 reserveWrite 本身是并行的，不能严格控制顺序，
	// 但最终结果必须是最小 TID 持有预留
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}),
		makeExecInfo(1, [2]string{"c", "k1"}),
		makeExecInfo(2, [2]string{"c", "k1"}),
	}

	table, aborted := reserveWrite(infos)

	val, _ := table.Load(constructKey("c", []byte("k1")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("final reservation holder = %d, want 0", res.txIndex)
	}
	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
}

// 空写集交易不影响其他交易
func TestReserveWrite_EmptyWriteSet(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0), // 空写集
		makeExecInfo(1, [2]string{"c", "k1"}),
	}

	_, aborted := reserveWrite(infos)

	if aborted[0].Load() {
		t.Error("tx0 (empty write set) should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted")
	}
}

// 不同 contract 的同名 key 不冲突
func TestReserveWrite_DifferentContracts_NoConflict(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"contractA", "k1"}),
		makeExecInfo(1, [2]string{"contractB", "k1"}),
	}

	_, aborted := reserveWrite(infos)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted")
	}
}

// 多 key 场景：tx0 写 k1,k2；tx1 写 k2,k3；tx2 写 k3,k4
func TestReserveWrite_MultiKeyChainConflict(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfo(0, [2]string{"c", "k1"}, [2]string{"c", "k2"}),
		makeExecInfo(1, [2]string{"c", "k2"}, [2]string{"c", "k3"}),
		makeExecInfo(2, [2]string{"c", "k3"}, [2]string{"c", "k4"}),
	}

	table, aborted := reserveWrite(infos)

	// tx0 不冲突
	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	// tx1 在 k2 上与 tx0 冲突
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted (k2 conflict with tx0)")
	}
	// tx2 在 k3 上与 tx1 冲突，但 tx2 > tx1，tx2 被 abort
	if !aborted[2].Load() {
		t.Error("tx2 should be aborted (k3 conflict with tx1)")
	}

	// k2 预留属于 tx0
	val, _ := table.Load(constructKey("c", []byte("k2")))
	if val.(*reservation).txIndex != 0 {
		t.Errorf("k2 holder = %d, want 0", val.(*reservation).txIndex)
	}
	// k3 预留属于 tx1
	val, _ = table.Load(constructKey("c", []byte("k3")))
	if val.(*reservation).txIndex != 1 {
		t.Errorf("k3 holder = %d, want 1", val.(*reservation).txIndex)
	}
	// k4 预留属于 tx2
	val, _ = table.Load(constructKey("c", []byte("k4")))
	if val.(*reservation).txIndex != 2 {
		t.Errorf("k4 holder = %d, want 2", val.(*reservation).txIndex)
	}
}

// 并发安全性压力测试：大量交易竞争同一个 key
func TestReserveWrite_ConcurrencyStress(t *testing.T) {
	const n = 100
	infos := make([]txExecInfo, n)
	for i := 0; i < n; i++ {
		infos[i] = makeExecInfo(i, [2]string{"c", "hot_key"})
	}

	table, aborted := reserveWrite(infos)

	// tx0 必须获胜
	if aborted[0].Load() {
		t.Fatal("tx0 should never be aborted")
	}

	// 其余所有交易都应被 abort
	for i := 1; i < n; i++ {
		if !aborted[i].Load() {
			t.Errorf("tx%d should be aborted", i)
		}
	}

	// 预留持有者必须是 tx0
	val, _ := table.Load(constructKey("c", []byte("hot_key")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("hot_key holder = %d, want 0", res.txIndex)
	}
}

// 验证 aborted 数组长度正确
func TestReserveWrite_AbortedLength(t *testing.T) {
	infos := make([]txExecInfo, 5)
	for i := range infos {
		infos[i] = makeExecInfo(i)
	}

	_, aborted := reserveWrite(infos)

	if len(aborted) != 5 {
		t.Errorf("aborted length = %d, want 5", len(aborted))
	}
}

// atomic.Bool 零值为 false 的假设验证
func TestAtomicBoolZeroValue(t *testing.T) {
	var b atomic.Bool
	if b.Load() {
		t.Error("atomic.Bool zero value should be false")
	}
}

// ==================== checkConflicts 测试 ====================
// Rule 2: abort if (1) WAW, or (2) both WAR and RAW dependencies exist.

// 无冲突场景：两个交易读写完全不重叠
func TestCheckConflicts_NoConflict(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoRW(0, nil, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k3"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted")
	}
}

// 仅 RAW 依赖（无 WAR）：tx1 读了 tx0 写的 key，但 tx1 写的 key 没有被更早交易读过
// Rule 2 下不应被 abort（需要同时有 WAR 和 RAW 才 abort）
func TestCheckConflicts_RAW_Only_NoAbort(t *testing.T) {
	// tx0 写 k1，tx1 读 k1（写 k2，但 tx0 没读 k2 → 无 WAR）
	infos := []txExecInfo{
		makeExecInfoRW(0, nil, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)

	// 写预留阶段两个交易写不同 key，都不会被 abort
	if aborted[0].Load() || aborted[1].Load() {
		t.Fatal("no tx should be aborted after reserveWrite (different write keys)")
	}

	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	// tx1 有 RAW（读 k1，tx0 写了 k1），但无 WAR → Rule 2 不 abort
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted (RAW only, no WAR)")
	}
}

// WAR + RAW 同时存在 → abort
func TestCheckConflicts_WAR_And_RAW_Abort(t *testing.T) {
	// tx0 读 k2，写 k1
	// tx1 读 k1（RAW：tx0 写了 k1），写 k2（WAR：tx0 读了 k2）
	// tx1 同时有 WAR 和 RAW → abort
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)

	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted (both WAR and RAW dependencies)")
	}
}

// 仅 WAR 依赖（无 RAW）→ 不 abort
func TestCheckConflicts_WAR_Only_NoAbort(t *testing.T) {
	// tx0 读 k2，写 k1
	// tx1 写 k2（WAR：tx0 读了 k2），读 k3（无 RAW：没有更早交易写 k3）
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k3"}}, [][2]string{{"c", "k2"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)

	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted (WAR only, no RAW)")
	}
}

// WAW 依赖：在 checkConflicts 中被检测（写预留阶段已标记的会跳过）
func TestCheckConflicts_WAW_AlreadyAbortedSkipped(t *testing.T) {
	// tx0 和 tx1 都写 k1，写预留阶段 tx1 已被 abort
	infos := []txExecInfo{
		makeExecInfoRW(0, nil, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, nil, [][2]string{{"c", "k1"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)

	// tx1 在写预留阶段已被 abort
	if !aborted[1].Load() {
		t.Fatal("tx1 should be aborted after reserveWrite")
	}

	checkConflicts(infos, table, readTable, aborted, nil)

	// tx0 仍然不被 abort
	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	// tx1 仍然是 aborted（被跳过，未改变）
	if !aborted[1].Load() {
		t.Error("tx1 should still be aborted")
	}
}

// RAW 依赖：tx0 读了自己写的 key（持有者是自己）→ 不应被 abort
func TestCheckConflicts_RAW_SelfWrite_NoAbort(t *testing.T) {
	// tx0 既读又写 k1（自己持有预留）
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k1"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	// tx0 的读集 k1 的预留持有者就是 tx0 自己，res.txIndex (0) 不 < idx (0)
	if aborted[0].Load() {
		t.Error("tx0 should not be aborted (reads its own write)")
	}
}

// 仅 RAW 链式依赖（无 WAR）：Rule 2 下均不 abort
func TestCheckConflicts_RAW_Chain_NoAbort(t *testing.T) {
	// tx0 写 k1，tx1 读 k1 写 k2，tx2 读 k2 写 k3
	// tx1 有 RAW（读 k1）但无 WAR（tx0 没读 k2）→ 不 abort
	// tx2 有 RAW（读 k2）但无 WAR（tx0/tx1 没读 k3）→ 不 abort
	infos := []txExecInfo{
		makeExecInfoRW(0, nil, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}),
		makeExecInfoRW(2, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k3"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted (RAW only, no WAR)")
	}
	if aborted[2].Load() {
		t.Error("tx2 should not be aborted (RAW only, no WAR)")
	}
}

// WAR+RAW 链式依赖
func TestCheckConflicts_WAR_RAW_Chain(t *testing.T) {
	// tx0 读 k2，写 k1
	// tx1 读 k1（RAW from tx0），写 k2（WAR: tx0 读了 k2）→ WAR+RAW → abort
	// tx2 读 k2（RAW from tx1? tx1 持有 k2 写预留? 不，tx0 不写 k2，tx1 写 k2，tx1 持有 k2 预留），
	//     写 k3（WAR: 需要更早的 tx 读了 k3，但没有）→ 仅 RAW，不 abort
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}),
		makeExecInfoRW(2, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k3"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted (WAR on k2 + RAW on k1)")
	}
	// tx2 读 k2，tx1 持有 k2 写预留（1<2）→ RAW。tx2 写 k3，无更早 tx 读 k3 → 无 WAR
	if aborted[2].Load() {
		t.Error("tx2 should not be aborted (RAW only, no WAR)")
	}
}

// 读一个不在预留表中的 key（没有交易写过）→ 不冲突
func TestCheckConflicts_RAW_ReadUnwrittenKey(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoRW(0, nil, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k_not_written"}}, [][2]string{{"c", "k2"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted (reads a key no one wrote)")
	}
}

// 后续交易写了某 key，前面交易读了该 key → 不算 RAW（只检查更早的写者）
func TestCheckConflicts_RAW_LaterWriterNoAbort(t *testing.T) {
	// tx0 读 k1，tx1 写 k1
	// tx1 持有 k1 预留，但 tx1.index(1) > tx0.index(0)，所以不构成 RAW
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}),
		makeExecInfoRW(1, nil, [][2]string{{"c", "k1"}}),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted (later tx1 wrote k1, not earlier)")
	}
	if aborted[1].Load() {
		t.Error("tx1 should not be aborted")
	}
}

// 综合场景：WAW + WAR+RAW 混合
func TestCheckConflicts_Mixed_WAW_WAR_RAW(t *testing.T) {
	// tx0 读 k3，写 k1, k2
	// tx1 写 k1（WAW 冲突，写预留阶段已 abort），读 k2
	// tx2 读 k1（RAW：tx0 持有 k1 预留），写 k3（WAR：tx0 读了 k3）→ WAR+RAW → abort
	// tx3 只读 k_none（不存在的 key，无冲突）
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k3"}}, [][2]string{{"c", "k1"}, {"c", "k2"}}),
		makeExecInfoRW(1, [][2]string{{"c", "k2"}}, [][2]string{{"c", "k1"}}),
		makeExecInfoRW(2, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k3"}}),
		makeExecInfoRW(3, [][2]string{{"c", "k_none"}}, nil),
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Error("tx0 should not be aborted")
	}
	// tx1 在写预留阶段已被 abort（WAW on k1），checkConflicts 跳过
	if !aborted[1].Load() {
		t.Error("tx1 should be aborted (WAW on k1)")
	}
	// tx2 有 RAW（读 k1，tx0 持有）+ WAR（写 k3，tx0 读了 k3）→ abort
	if !aborted[2].Load() {
		t.Error("tx2 should be aborted (WAR on k3 + RAW on k1)")
	}
	// tx3 只读不存在的 key，无冲突
	if aborted[3].Load() {
		t.Error("tx3 should not be aborted")
	}
}

// ==================== reserveRead 测试 ====================

// helper：快速构造只有读集的 txExecInfo
func makeExecInfoReadOnly(index int, reads ...[2]string) txExecInfo {
	var txReads []*commonPb.TxRead
	for _, r := range reads {
		txReads = append(txReads, &commonPb.TxRead{
			ContractName: r[0],
			Key:          []byte(r[1]),
		})
	}
	return txExecInfo{
		index:     index,
		txReadSet: txReads,
	}
}

// 单个交易读一个 key
func TestReserveRead_SingleTx(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"c", "k1"}),
	}

	table := reserveRead(infos)

	val, ok := table.Load(constructKey("c", []byte("k1")))
	if !ok {
		t.Fatal("read reservation for c+k1 not found")
	}
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("read reservation txIndex = %d, want 0", res.txIndex)
	}
}

// 两个交易读不同 key，各自持有预留
func TestReserveRead_NoOverlap(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"c", "k1"}),
		makeExecInfoReadOnly(1, [2]string{"c", "k2"}),
	}

	table := reserveRead(infos)

	val, _ := table.Load(constructKey("c", []byte("k1")))
	if val.(*reservation).txIndex != 0 {
		t.Errorf("k1 holder = %d, want 0", val.(*reservation).txIndex)
	}
	val, _ = table.Load(constructKey("c", []byte("k2")))
	if val.(*reservation).txIndex != 1 {
		t.Errorf("k2 holder = %d, want 1", val.(*reservation).txIndex)
	}
}

// 两个交易读同一个 key：最小 TID 持有预留，且不标记 abort
func TestReserveRead_SameKey_SmallerWins_NoAbort(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"c", "k1"}),
		makeExecInfoReadOnly(1, [2]string{"c", "k1"}),
	}

	table := reserveRead(infos)

	val, ok := table.Load(constructKey("c", []byte("k1")))
	if !ok {
		t.Fatal("read reservation not found")
	}
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("read reservation holder = %d, want 0 (smallest TID)", res.txIndex)
	}
}

// 三个交易竞争同一个 key：最小 TID 获胜
func TestReserveRead_ThreeWayCompetition(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"c", "k1"}),
		makeExecInfoReadOnly(1, [2]string{"c", "k1"}),
		makeExecInfoReadOnly(2, [2]string{"c", "k1"}),
	}

	table := reserveRead(infos)

	val, _ := table.Load(constructKey("c", []byte("k1")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("read reservation holder = %d, want 0", res.txIndex)
	}
}

// 空读集的交易不影响预留表
func TestReserveRead_EmptyReadSet(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0), // 空读集
		makeExecInfoReadOnly(1, [2]string{"c", "k1"}),
	}

	table := reserveRead(infos)

	val, ok := table.Load(constructKey("c", []byte("k1")))
	if !ok {
		t.Fatal("read reservation for k1 not found")
	}
	if val.(*reservation).txIndex != 1 {
		t.Errorf("k1 holder = %d, want 1", val.(*reservation).txIndex)
	}
}

// 不同 contract 的同名 key 不冲突
func TestReserveRead_DifferentContracts(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"contractA", "k1"}),
		makeExecInfoReadOnly(1, [2]string{"contractB", "k1"}),
	}

	table := reserveRead(infos)

	valA, okA := table.Load(constructKey("contractA", []byte("k1")))
	valB, okB := table.Load(constructKey("contractB", []byte("k1")))
	if !okA || !okB {
		t.Fatal("reservations not found")
	}
	if valA.(*reservation).txIndex != 0 {
		t.Errorf("contractA+k1 holder = %d, want 0", valA.(*reservation).txIndex)
	}
	if valB.(*reservation).txIndex != 1 {
		t.Errorf("contractB+k1 holder = %d, want 1", valB.(*reservation).txIndex)
	}
}

// 多 key 场景：tx0 读 k1,k2；tx1 读 k2,k3；tx2 读 k3,k4
func TestReserveRead_MultiKeyOverlap(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoReadOnly(0, [2]string{"c", "k1"}, [2]string{"c", "k2"}),
		makeExecInfoReadOnly(1, [2]string{"c", "k2"}, [2]string{"c", "k3"}),
		makeExecInfoReadOnly(2, [2]string{"c", "k3"}, [2]string{"c", "k4"}),
	}

	table := reserveRead(infos)

	// k1 → tx0
	val, _ := table.Load(constructKey("c", []byte("k1")))
	if val.(*reservation).txIndex != 0 {
		t.Errorf("k1 holder = %d, want 0", val.(*reservation).txIndex)
	}
	// k2 → tx0（tx0 和 tx1 都读 k2，tx0 更小）
	val, _ = table.Load(constructKey("c", []byte("k2")))
	if val.(*reservation).txIndex != 0 {
		t.Errorf("k2 holder = %d, want 0", val.(*reservation).txIndex)
	}
	// k3 → tx1（tx1 和 tx2 都读 k3，tx1 更小）
	val, _ = table.Load(constructKey("c", []byte("k3")))
	if val.(*reservation).txIndex != 1 {
		t.Errorf("k3 holder = %d, want 1", val.(*reservation).txIndex)
	}
	// k4 → tx2
	val, _ = table.Load(constructKey("c", []byte("k4")))
	if val.(*reservation).txIndex != 2 {
		t.Errorf("k4 holder = %d, want 2", val.(*reservation).txIndex)
	}
}

// 并发压力测试：100 个交易竞争同一个 key，最小 TID 获胜
func TestReserveRead_ConcurrencyStress(t *testing.T) {
	const n = 100
	infos := make([]txExecInfo, n)
	for i := 0; i < n; i++ {
		infos[i] = makeExecInfoReadOnly(i, [2]string{"c", "hot_key"})
	}

	table := reserveRead(infos)

	// 预留持有者必须是 tx0
	val, _ := table.Load(constructKey("c", []byte("hot_key")))
	res := val.(*reservation)
	if res.txIndex != 0 {
		t.Errorf("hot_key holder = %d, want 0", res.txIndex)
	}
}

// 使用 makeExecInfoRW 构造带读写集的交易，验证 reserveRead 只关注读集
func TestReserveRead_IgnoresWriteSet(t *testing.T) {
	infos := []txExecInfo{
		makeExecInfoRW(0, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k2"}}), // 读 k1，写 k2
		makeExecInfoRW(1, [][2]string{{"c", "k1"}}, [][2]string{{"c", "k3"}}), // 读 k1，写 k3
	}

	table := reserveRead(infos)

	// k1 在读预留表中，持有者为 tx0
	val, ok := table.Load(constructKey("c", []byte("k1")))
	if !ok {
		t.Fatal("read reservation for k1 not found")
	}
	if val.(*reservation).txIndex != 0 {
		t.Errorf("k1 holder = %d, want 0", val.(*reservation).txIndex)
	}

	// k2, k3 是写集，不应出现在读预留表中
	_, okK2 := table.Load(constructKey("c", []byte("k2")))
	_, okK3 := table.Load(constructKey("c", []byte("k3")))
	if okK2 {
		t.Error("k2 should not be in read reserve table (it's a write key)")
	}
	if okK3 {
		t.Error("k3 should not be in read reserve table (it's a write key)")
	}
}

// 并发压力测试：仅 RAW（无 WAR）→ Rule 2 下不 abort
func TestCheckConflicts_ConcurrencyStress_RAW_Only_NoAbort(t *testing.T) {
	const n = 100
	infos := make([]txExecInfo, n)
	// tx0 写 hot_key
	infos[0] = makeExecInfoRW(0, nil, [][2]string{{"c", "hot_key"}})
	// tx1~tx99 读 hot_key，写各自不同的 key（tx0 没读那些 key → 无 WAR）
	for i := 1; i < n; i++ {
		infos[i] = makeExecInfoRW(i,
			[][2]string{{"c", "hot_key"}},
			[][2]string{{"c", string(rune('a' + i))}}, // 不同 key 避免 WAW
		)
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Fatal("tx0 should not be aborted")
	}
	for i := 1; i < n; i++ {
		if aborted[i].Load() {
			t.Errorf("tx%d should not be aborted (RAW only, no WAR under Rule 2)", i)
		}
	}
}

// 并发压力测试：WAR + RAW → abort
func TestCheckConflicts_ConcurrencyStress_WAR_RAW_Abort(t *testing.T) {
	const n = 100
	infos := make([]txExecInfo, n)
	// tx0 读 all unique keys，写 hot_key
	uniqueReads := make([][2]string, n-1)
	for i := 1; i < n; i++ {
		uniqueReads[i-1] = [2]string{"c", string(rune('a' + i))}
	}
	infos[0] = makeExecInfoRW(0, uniqueReads, [][2]string{{"c", "hot_key"}})
	// tx1~tx99 读 hot_key（RAW from tx0），写各自的 unique key（WAR: tx0 读了该 key）
	for i := 1; i < n; i++ {
		infos[i] = makeExecInfoRW(i,
			[][2]string{{"c", "hot_key"}},
			[][2]string{{"c", string(rune('a' + i))}},
		)
	}

	table, aborted := reserveWrite(infos)
	readTable := reserveRead(infos)
	checkConflicts(infos, table, readTable, aborted, nil)

	if aborted[0].Load() {
		t.Fatal("tx0 should not be aborted")
	}
	for i := 1; i < n; i++ {
		if !aborted[i].Load() {
			t.Errorf("tx%d should be aborted (WAR + RAW under Rule 2)", i)
		}
	}
}
