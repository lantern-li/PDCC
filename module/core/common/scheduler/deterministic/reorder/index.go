package reorder

import (
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/utils"
	"chainmaker.org/chainmaker/common/v2/bitmap"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// TxIndex tx with index in block
type TxIndex struct {
	Index int
	Tx    *commonPb.Transaction
}

// TxResultIndex has all results from function ExecuteTx
type TxResultIndex struct {
	Index   int
	Sim     protocol.TxSimContext
	TxType  protocol.ExecOrderTxType
	Success bool
}

type cycleCountIndex struct {
	i     int
	count int
}

// RWSet2Bitmap from TxResultIndex to rw set bitmap
func RWSet2Bitmap(txResults []*TxResultIndex) ([]*bitmap.Bitmap, []*bitmap.Bitmap) {
	readMap := make([]*bitmap.Bitmap, len(txResults))
	writeMap := make([]*bitmap.Bitmap, len(txResults))
	keyMap := make(map[string]int)
	keyCount := 0
	for i, txResult := range txResults {
		readMap[i] = &bitmap.Bitmap{}
		writeMap[i] = &bitmap.Bitmap{}
		// if no result, we ignore it, and handle it later in graph
		if txResult == nil {
			continue
		}
		txRWSet := txResult.Sim.GetTxRWSet(txResult.Success)
		for _, e := range txRWSet.GetTxReads() {
			finalKey := utils.ConstructKey(e.GetContractName(), e.GetKey())
			if _keyCount, ok := keyMap[finalKey]; ok {
				readMap[i].Set(_keyCount)
			} else {
				keyMap[finalKey] = keyCount
				readMap[i].Set(keyCount)
				keyCount++
			}
		}
		for _, e := range txRWSet.GetTxWrites() {
			finalKey := utils.ConstructKey(e.GetContractName(), e.GetKey())
			if _keyCount, ok := keyMap[finalKey]; ok {
				writeMap[i].Set(_keyCount)
			} else {
				keyMap[finalKey] = keyCount
				writeMap[i].Set(keyCount)
				keyCount++
			}
		}
	}
	return readMap, writeMap
}

// RWSet2Map from TxResultIndex to rw set map
func RWSet2Map(txResults []*TxResultIndex) (map[string][]int, map[string][]int) {
	readMap := make(map[string][]int)
	writeMap := make(map[string][]int)
	for i, txResult := range txResults {
		// if no result, we ignore it, and handle it later in graph
		if txResult == nil {
			continue
		}
		txRWSet := txResult.Sim.GetTxRWSet(txResult.Success)
		for _, e := range txRWSet.GetTxReads() {
			finalKey := utils.ConstructKey(e.GetContractName(), e.GetKey())
			if list, ok := readMap[finalKey]; ok {
				readMap[finalKey] = append(list, i)
			} else {
				readMap[finalKey] = []int{i}
			}
		}
		for _, e := range txRWSet.GetTxWrites() {
			finalKey := utils.ConstructKey(e.GetContractName(), e.GetKey())
			if list, ok := writeMap[finalKey]; ok {
				writeMap[finalKey] = append(list, i)
			} else {
				writeMap[finalKey] = []int{i}
			}
		}
	}
	return readMap, writeMap
}
