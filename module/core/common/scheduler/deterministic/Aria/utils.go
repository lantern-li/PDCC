package aria

import (
	"sync"
	"sync/atomic"
)

// reservation 表示一个 key 的写预留条目
type reservation struct {
	mu      sync.Mutex
	txIndex int // 当前持有预留的交易序号（TID）
}

// constructKey construct keys: contractName#key
func constructKey(contractName string, key []byte) string {
	// with higher performance
	return contractName + string(key)
	// var builder strings.Builder
	// builder.WriteString(contractName)
	// builder.Write(key)
	// return builder.String()
}

// hasWAWConflict 检查交易 idx 是否存在 WAW（Write-After-Write）依赖。
// 遍历写集，若某个 key 的预留持有者不是自己（必定是更小的 TID），则存在 WAW 依赖。
func hasWAWConflict(info *txExecInfo, idx int, reserveTable *sync.Map) bool {
	for _, txWrite := range info.txWriteSet {
		key := constructKey(txWrite.ContractName, txWrite.Key)
		val, ok := reserveTable.Load(key)
		if !ok {
			continue
		}
		res := val.(*reservation)
		if res.txIndex != idx {
			return true
		}
	}
	return false
}

// hasRAWConflict 检查交易 idx 是否存在 RAW（Read-After-Write）依赖。
// 遍历读集，若某个更早的交易持有该 key 的写预留，则存在 RAW 依赖。
func hasRAWConflict(info *txExecInfo, idx int, reserveTable *sync.Map) bool {
	for _, txRead := range info.txReadSet {
		key := constructKey(txRead.ContractName, txRead.Key)
		val, ok := reserveTable.Load(key)
		if !ok {
			continue
		}
		res := val.(*reservation)
		if res.txIndex < idx {
			return true
		}
	}
	return false
}

// checkConflicts 冲突检查阶段：基于预留表检查 WAW 和 RAW 依赖，决定交易能否提交。
//
// 根据 Aria 论文 Rule 1：一个交易可以提交，当且仅当它对所有更早的交易（∀j<i）
// 没有 WAW 依赖和 RAW 依赖。
//
// 检查方式（通过探查写预留表，而非遍历所有更早交易）：
//   - WAW 检查：对 Ti 写集中的每个 key，查预留表。若持有者不是 Ti 自己，
//     说明某个更早的 Tj 也写了该 key → Ti 有 WAW 依赖 → abort。
//   - RAW 检查：对 Ti 读集中的每个 key，查预留表。若某个 Tj (j<i) 持有预留，
//     说明 Ti 应该看到 Tj 的写入但实际没有（都在相同 snapshot 上执行） → Ti 有 RAW 依赖 → abort。
//
// 已在写预留阶段被 abort 的交易可跳过此阶段（性能优化）。
// 此阶段可并行执行，顺序无关。
func checkConflicts(execInfos []txExecInfo, reserveTable *sync.Map, aborted []atomic.Bool) {
	var wg sync.WaitGroup
	for i := range execInfos {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// 已在写预留阶段被 abort 的交易可跳过
			if aborted[idx].Load() {
				return
			}

			info := &execInfos[idx]

			// WAW 检查
			if hasWAWConflict(info, idx, reserveTable) {
				aborted[idx].Store(true)
				return
			}

			// RAW 检查
			if hasRAWConflict(info, idx, reserveTable) {
				aborted[idx].Store(true)
				return
			}
		}(i)
	}
	wg.Wait()
}

// reserveRead 读预留阶段：每个交易遍历自己的读集，对每个 key 进行预留。
// 规则：
//   - 只有 TID 更小的交易才能覆盖已有预留
//   - 与写预留不同，这里不标记任何交易为 abort，仅记录最小 TID
//   - 用于后续 WAR（Write-After-Read）冲突检测
func reserveRead(execInfos []txExecInfo) *sync.Map {
	readReserveTable := &sync.Map{} // key: string(contractName + key) → *reservation

	var wg sync.WaitGroup
	for i := range execInfos {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			info := &execInfos[idx]

			for _, txRead := range info.txReadSet {
				reserveKey := constructKey(txRead.ContractName, txRead.Key)

				// 获取或创建该 key 的预留条目
				val, _ := readReserveTable.LoadOrStore(reserveKey, &reservation{txIndex: idx})
				res := val.(*reservation)

				res.mu.Lock()
				if res.txIndex == idx {
					// 我们刚刚创建的预留成功
				} else if idx < res.txIndex {
					// 当前 TID 更小，覆盖已有预留
					res.txIndex = idx
				}
				// 不需要标记 abort，仅保留最小 TID
				res.mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	return readReserveTable
}

// reserveWrite 写预留阶段：每个交易遍历自己的写集，对每个 key 进行预留。
// 规则：
//   - 只有 TID 更小的交易才能覆盖已有预留
//   - 如果已有预留的 TID 更小，则当前交易预留失败，标记为 abort
//   - 即使某个 key 预留失败，交易也必须继续预留剩余 key（因为并行执行，省略会导致非确定性）
func reserveWrite(execInfos []txExecInfo) (*sync.Map, []atomic.Bool) {
	reserveTable := &sync.Map{} // key: string(contractName + key) → *reservation
	aborted := make([]atomic.Bool, len(execInfos))

	var reserveWg sync.WaitGroup
	for i := range execInfos { // refactor:原文这里多次强调了需要这一步需要并行。 但是根据我的实际经验，这里串行的做似乎会更加好，但还是尊重原文吧。
		reserveWg.Add(1)
		go func(idx int) {
			defer reserveWg.Done()
			info := &execInfos[idx] // comment：这里用指针
			mustAbort := false

			for _, txWrite := range info.txWriteSet {
				reserveKey := constructKey(txWrite.ContractName, txWrite.Key)

				// 获取或创建该 key 的预留条目
				val, _ := reserveTable.LoadOrStore(reserveKey, &reservation{txIndex: idx})
				res := val.(*reservation)

				res.mu.Lock()
				if res.txIndex == idx {
					// 我们刚刚创建的预留成功
				} else if idx < res.txIndex {
					// 当前 TID 更小，覆盖已有预留，旧持有者需要 abort
					aborted[res.txIndex].Store(true)
					res.txIndex = idx
				} else {
					// 已有预留的 TID 更小，当前交易预留失败，但必须继续预留剩余 key
					mustAbort = true
				}
				res.mu.Unlock()
			}

			if mustAbort {
				aborted[idx].Store(true)
			}
		}(i)
	}
	reserveWg.Wait()

	return reserveTable, aborted
}
