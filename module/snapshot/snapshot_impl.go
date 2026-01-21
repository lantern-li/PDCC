/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package snapshot

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"

	"chainmaker.org/chainmaker/common/v2/bitmap"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	vmPb "chainmaker.org/chainmaker/pb-go/v2/vm"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
)

// The record value is written by the SEQ corresponding to TX
type sv struct {
	seq   int
	value []byte
}

type SnapshotImpl struct {
	lock            sync.RWMutex
	tableLock       sync.RWMutex
	blockchainStore protocol.BlockchainStore
	log             protocol.Logger
	// If the snapshot has been sealed, the results of subsequent vm execution will not be added to the snapshot
	sealed *atomic.Bool

	chainId        string
	blockTimestamp int64
	blockProposer  *accesscontrol.Member
	blockHeight    uint64
	blockVersion   uint32
	preBlockHash   []byte

	preSnapshot protocol.Snapshot

	blockFingerprint string
	lastChainConfig  *config.ChainConfig

	// applied data, please lock it before using
	txRWSetTable   []*commonPb.TxRWSet
	txTable        []*commonPb.Transaction
	specialTxTable []*commonPb.Transaction
	txResultMap    map[string]*commonPb.Result

	// 调整为分片map
	readTable  *ShardSet
	writeTable *ShardSet

	applyConflictTime *atomic.Int64
	applyAddReadTime  *atomic.Int64
	applyAddWriteTime *atomic.Int64

	txRoot    []byte
	dagHash   []byte
	rwSetHash []byte

	txLock    sync.Mutex // 交易读写kv锁
	txLockLog sync.Map   // txId => 交易读写锁（ps: 与txLockMap中的锁是同一把锁)
}

type txLocker struct {
	sync.Mutex
	lockerName string
}

// NewQuerySnapshot create a snapshot for query tx
func NewQuerySnapshot(store protocol.BlockchainStore, log protocol.Logger) (*SnapshotImpl, error) {
	txCount := 1
	lastBlock, err := store.GetLastBlock()
	if err != nil {
		return nil, err
	}
	lastChainConfig, err := store.GetLastChainConfig()
	if err != nil || lastChainConfig == nil {
		return nil, fmt.Errorf("failed to get last chain config, %v", err)
	}

	querySnapshot := &SnapshotImpl{
		blockchainStore: store,
		preSnapshot:     nil,
		log:             log,
		txResultMap:     make(map[string]*commonPb.Result, txCount),

		chainId:         lastBlock.Header.ChainId,
		blockHeight:     lastBlock.Header.BlockHeight,
		blockVersion:    lastBlock.Header.BlockVersion,
		blockTimestamp:  lastBlock.Header.BlockTimestamp,
		blockProposer:   lastBlock.Header.Proposer,
		preBlockHash:    lastBlock.Header.PreBlockHash,
		lastChainConfig: lastChainConfig,

		txTable:      make([]*commonPb.Transaction, 0, txCount),
		txRWSetTable: make([]*commonPb.TxRWSet, 0, txCount*10),

		readTable:  newShardSet(),
		writeTable: newShardSet(),

		applyConflictTime: atomic.NewInt64(0),
		applyAddReadTime:  atomic.NewInt64(0),
		applyAddWriteTime: atomic.NewInt64(0),

		txRoot:    lastBlock.Header.TxRoot,
		dagHash:   lastBlock.Header.DagHash,
		rwSetHash: lastBlock.Header.RwSetRoot,
	}

	return querySnapshot, nil
}

// GetPreSnapshot previous snapshot
func (s *SnapshotImpl) GetPreSnapshot() protocol.Snapshot {
	return s.preSnapshot
}

// SetPreSnapshot previous snapshot
func (s *SnapshotImpl) SetPreSnapshot(snapshot protocol.Snapshot) {
	s.preSnapshot = snapshot
}

// GetBlockchainStore return the blockchainStore of the snapshot
func (s *SnapshotImpl) GetBlockchainStore() protocol.BlockchainStore {
	return s.blockchainStore
}

// GetLastChainConfig return the last chain config
func (s *SnapshotImpl) GetLastChainConfig() *config.ChainConfig {
	return s.lastChainConfig
}

// GetSnapshotSize return the len of the txTable
func (s *SnapshotImpl) GetSnapshotSize() int {
	s.tableLock.RLock()
	defer s.tableLock.RUnlock()
	return len(s.txTable)
}

// GetTxTable return the txTable of the snapshot
func (s *SnapshotImpl) GetTxTable() []*commonPb.Transaction {
	return s.txTable
}

// GetSpecialTxTable return the specialTxTable of the snapshot
func (s *SnapshotImpl) GetSpecialTxTable() []*commonPb.Transaction {
	return s.specialTxTable
}

// GetTxResultMap After the scheduling is completed, get the result from the current snapshot
func (s *SnapshotImpl) GetTxResultMap() map[string]*commonPb.Result {
	return s.txResultMap
}

// GetTxRWSetTable return the snapshot's txRWSetTable
func (s *SnapshotImpl) GetTxRWSetTable() []*commonPb.TxRWSet {
	if localconf.ChainMakerConfig.SchedulerConfig.RWSetLog {
		s.log.DebugDynamic(func() string {
			info := "rwset: "
			for i, txRWSet := range s.txRWSetTable {
				info += fmt.Sprintf("read set for tx id:[%s], count [%d]<", s.txTable[i].Payload.TxId, len(txRWSet.TxReads))
				//for _, txRead := range txRWSet.TxReads {
				//	if !strings.HasPrefix(string(txRead.Key), protocol.ContractByteCode) {
				//		info += fmt.Sprintf("[%v] -> [%v], contract name [%v], version [%v],",
				//		txRead.Key, txRead.Value, txRead.ContractName, txRead.Version)
				//	}
				//}
				info += "> "
				info += fmt.Sprintf("write set for tx id:[%s], count [%d]<", s.txTable[i].Payload.TxId, len(txRWSet.TxWrites))
				for _, txWrite := range txRWSet.TxWrites {
					info += fmt.Sprintf("[%v] -> [%v], contract name [%v], ", txWrite.Key, txWrite.Value, txWrite.ContractName)
				}
				info += ">"
			}
			return info
		})
		// log.Debugf(info)
	}

	//for _, txRWSet := range s.txRWSetTable {
	//	for _, txRead := range txRWSet.TxReads {
	//		if strings.HasPrefix(string(txRead.Key), protocol.ContractByteCode) ||
	//			strings.HasPrefix(string(txRead.Key), protocol.ContractCreator) ||
	//			txRead.ContractName == syscontract.SystemContract_CERT_MANAGE.String() {
	//			txRead.Value = nil
	//		}
	//	}
	//}
	return s.txRWSetTable
}

// Lock lock snapshot with locker name
func (s *SnapshotImpl) Lock(lockerName, txId string) error {
	/**
	1. 根据lockerName检查锁是否存在
	2. 根据txId判断是否正在锁中
	3. 对于locker进行加锁
	4. GetKey()
	*/

	// 获取合约关键字锁（如果不存在，则构造）
	// txLockMapLockStartStick := utils.CurrentTimeMillisSeconds()
	// s.txLockMapLock.RLock()
	// locker, ok := s.txLockMap[lockerName]
	// s.txLockMapLock.RUnlock()
	// txLockMapLockCost := utils.CurrentTimeMillisSeconds() - txLockMapLockStartStick

	//newTxLockMapStartTick := utils.CurrentTimeMillisSeconds()
	//if !ok {
	//	s.txLockMapLock.Lock()
	//	locker, ok = s.txLockMap[lockerName] // Double-Checked Locking
	//	if !ok {
	//		locker = &txLocker{
	//			lockerName: lockerName,
	//		}
	//		s.txLockMap[lockerName] = locker
	//	}
	//	s.txLockMapLock.Unlock()
	//}
	//newTxLockMapCost := utils.CurrentTimeMillisSeconds() - newTxLockMapStartTick

	// 检查当前交易是否正在使用锁,如果未使用，则合约关键字锁进行加锁操作，然后记录txid
	_, ok := s.txLockLog.LoadOrStore(txId, struct{}{}) // 这么设计是由于目前只允许一个交易一个locker
	// getLockStartTick := utils.CurrentTimeMillisSeconds()
	if !ok {
		s.txLock.Lock() // ps: 这笔交易在apply时才进行解锁操作。
	}
	// getLockCost := utils.CurrentTimeMillisSeconds() - getLockStartTick

	//s.log.DebugDynamic(func() string {
	//	return fmt.Sprintf("snapshot lock success[txId:%s, txLockMapLockCost:%v, newTxLockMapCost:%v, getLockCost:%v]",
	//		txId, txLockMapLockCost, newTxLockMapCost, getLockCost)
	//})

	return nil
}

// GetKeyWithLock from snapshot with lock
func (s *SnapshotImpl) GetKeyWithLock(txExecSeq int, contractName, lockerName, txId string, key []byte) ([]byte, error) {
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("GetKeyWithLock contractName:%s, lockerName:%s, txId:%s, height:%d",
			contractName, lockerName, txId, s.blockHeight)
	})

	// getLockStartTick := utils.CurrentTimeMillisSeconds()
	err := s.Lock(lockerName, txId)
	if err != nil {
		s.log.Warnf("snapshot GetKeyWithLock fail[contract:%s, txid:%s, lock:%s]",
			contractName, txId, lockerName)
		return nil, err
	}
	// getLockCost := utils.CurrentTimeMillisSeconds() - getLockStartTick

	// getKeyStartTick := utils.CurrentTimeMillisSeconds()
	v, err := s.GetKey(txExecSeq, contractName, key)
	if err != nil {
		s.log.Warnf("snapshot GetKeyWithLock fail[contract:%s, txid:%s, lock:%s]",
			contractName, txId, lockerName)
		return nil, err
	}
	//getKeyCost := utils.CurrentTimeMillisSeconds() - getKeyStartTick
	//
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("GetKeyWithLock success[contractName:%s, lockerName:%s, txId:%s, height:%d]",
			contractName, lockerName, txId, s.blockHeight)
	})

	return v, nil
}

// GetKey from snapshot
func (s *SnapshotImpl) GetKey(txExecSeq int, contractName string, key []byte) ([]byte, error) {
	// get key before txExecSeq
	// snapshotSize := s.GetSnapshotSize()

	//s.lock.RLock()
	//defer s.lock.RUnlock()
	//if txExecSeq > snapshotSize || txExecSeq < 0 {
	//	txExecSeq = snapshotSize //nolint: ineffassign, staticcheck
	//}
	finalKey := constructKey(contractName, key)
	if sv, ok := s.writeTable.getByLock(finalKey); ok {
		return sv.value, nil
	}
	if sv, ok := s.readTable.getByLock(finalKey); ok {
		return sv.value, nil
	}

	iter := s.preSnapshot
	for iter != nil {
		if value, err := iter.GetKey(-1, contractName, key); err == nil {
			return value, nil
		}
		iter = iter.GetPreSnapshot()
	}

	return s.blockchainStore.ReadObject(contractName, key)
}

// GetKeys from snapshot
func (s *SnapshotImpl) GetKeys(txExecSeq int, keys []*vmPb.BatchKey) ([]*vmPb.BatchKey, error) {
	var (
		done              bool
		err               error
		writeSetValues    []*vmPb.BatchKey
		readSetValues     []*vmPb.BatchKey
		emptyWriteSetKeys []*vmPb.BatchKey
		emptyReadSetKeys  []*vmPb.BatchKey
		value             []*vmPb.BatchKey
	)
	// get key before txExecSeq
	//snapshotSize := s.GetSnapshotSize()
	//
	////s.lock.RLock()
	////defer s.lock.RUnlock()
	//if txExecSeq > snapshotSize || txExecSeq < 0 {
	//	txExecSeq = snapshotSize //nolint: ineffassign, staticcheck
	//}

	if writeSetValues, emptyWriteSetKeys, done = s.getBatchFromWriteSet(keys); done {
		return writeSetValues, nil
	}

	if readSetValues, emptyReadSetKeys, done = s.getBatchFromReadSet(emptyWriteSetKeys); done { //todo：这里的逻辑是对的，是优化的点。
		return append(readSetValues, writeSetValues...), nil
	}

	iter := s.preSnapshot
	for iter != nil {
		if value, err = iter.GetKeys(-1, emptyReadSetKeys); err == nil {
			return append(value, append(readSetValues, writeSetValues...)...), nil
		}
		iter = iter.GetPreSnapshot()
	}

	objects, err := s.getObjects(emptyReadSetKeys)
	if err != nil {
		return nil, err
	}
	return append(objects, append(value, append(readSetValues, writeSetValues...)...)...), nil
}

// GetKeysWithLock from snapshot
func (s *SnapshotImpl) GetKeysWithLock(txExecSeq int, keys []*vmPb.BatchKey, lockerName, txId string) ([]*vmPb.BatchKey, error) {
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("GetKeysWithLock lockerName:%s, txId:%s, height:%d", lockerName, txId, s.blockHeight)
	})
	getLockStartTick := time.Now().UnixMicro()
	err := s.Lock(lockerName, txId)
	if err != nil {
		s.log.Warnf("snapshot GetKeyWithLock fail[txid:%s, lock:%s]",
			txId, lockerName)
		return nil, err
	}
	getLockCost := time.Now().UnixMicro() - getLockStartTick
	//

	getKeysStartTick := time.Now().UnixMicro()
	batchKeys, err := s.GetKeys(txExecSeq, keys)
	getKeysCost := time.Now().UnixMicro() - getKeysStartTick

	timeLog := fmt.Sprintf("GetKeysWithLock success[lockerName:%s, txId:%s, height:%d, lockCost:%d us, getKeysCost:%d us, currentTime:%d]",
		lockerName, txId, s.blockHeight, getLockCost, getKeysCost, time.Now().UnixMicro())

	s.log.DebugDynamic(func() string {
		return timeLog
	})

	return batchKeys, err
}

// getObjects returns objects on given keys
func (s *SnapshotImpl) getObjects(keys []*vmPb.BatchKey) ([]*vmPb.BatchKey, error) {
	var contractName string
	if len(keys) > 0 {
		contractName = keys[0].ContractName
	}
	// index keys
	indexKeys := make(map[int]*vmPb.BatchKey, len(keys))
	res := make([]*vmPb.BatchKey, 0, len(keys))
	inputKeys := make([][]byte, 0, len(keys))
	for i, key := range keys {
		indexKeys[i] = key
		inputKeys = append(inputKeys, protocol.GetKeyStr(key.Key, key.Field))
	}

	readObjects, err := s.blockchainStore.ReadObjects(contractName, inputKeys)
	if err != nil {
		return nil, err
	}

	// construct keys from read objects and index keys
	for i, value := range readObjects {
		key := indexKeys[i]
		key.Value = value
		res = append(res, key)
	}
	return res, nil
}

// getBatchFromWriteSet  getBatchFromWriteSet
func (s *SnapshotImpl) getBatchFromWriteSet(keys []*vmPb.BatchKey) ([]*vmPb.BatchKey,
	[]*vmPb.BatchKey, bool) {
	txWrites := make([]*vmPb.BatchKey, 0, len(keys))
	emptyTxWrite := make([]*vmPb.BatchKey, 0, len(keys))
	for _, key := range keys {
		finalKey := constructKey(key.ContractName, protocol.GetKeyStr(key.Key, key.Field))
		if sv, ok := s.writeTable.getByLock(finalKey); ok {
			key.Value = sv.value
			txWrites = append(txWrites, key)
		} else {
			emptyTxWrite = append(emptyTxWrite, key)
		}
	}

	if len(emptyTxWrite) == 0 {
		return txWrites, nil, true
	}
	return txWrites, emptyTxWrite, false
}

// getBatchFromReadSet  getBatchFromReadSet
func (s *SnapshotImpl) getBatchFromReadSet(keys []*vmPb.BatchKey) ([]*vmPb.BatchKey,
	[]*vmPb.BatchKey, bool) {
	txReads := make([]*vmPb.BatchKey, 0, len(keys))
	emptyTxReadsKeys := make([]*vmPb.BatchKey, 0, len(keys))
	for _, key := range keys {
		finalKey := constructKey(key.ContractName, protocol.GetKeyStr(key.Key, key.Field))
		if sv, ok := s.readTable.getByLock(finalKey); ok {
			key.Value = sv.value
			txReads = append(txReads, key)
		} else {
			emptyTxReadsKeys = append(emptyTxReadsKeys, key)
		}
	}

	if len(emptyTxReadsKeys) == 0 {
		return txReads, nil, true
	}
	return txReads, emptyTxReadsKeys, false
}

// applyOptimize apply the normal tx to snapshot
func (s *SnapshotImpl) applyNormalTxSimContext(tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, runVmSuccess bool) (bool, int) {
	// 税总合约从这退出
	if _, ok := SZContractList[tx.Payload.ContractName]; ok {
		return s.dealSZTx(txSimContext, protocol.ExecOrderTxTypeNormal,
			runVmSuccess, false, tx)
	}

	if s.IsSealed() {
		return false, s.GetSnapshotSize()
	}
	// 乐观处理，以所有交易都不冲突的情况进行优先处理
	txExecSeq := txSimContext.GetTxExecSeq()
	var txRWSet *commonPb.TxRWSet
	var txResult *commonPb.Result

	// Only when the virtual machine is running normally can the read-write set be saved, or write fake conflicted key
	txRWSet = txSimContext.GetTxRWSet(runVmSuccess) //执行失败的交易只会返回其读集
	s.log.Debugf("【gas calc】%v, ApplyTxSimContext, txRWSet = %v", txSimContext.GetTx().Payload.TxId, txRWSet)
	txResult = txSimContext.GetTxResult()
	// 提前准备好要处理的数据
	finalReadKvs := make(map[string]*sv, len(txRWSet.TxReads))
	for _, txRead := range txRWSet.TxReads {
		finalKey := constructKey(txRead.ContractName, txRead.Key)
		// 乐观检查，便于提前发现冲突
		if sv, ok := s.writeTable.getByLock(finalKey); ok {
			if sv.seq >= txExecSeq {
				s.log.Debugf("Key Conflicted %+v-%+v, tx id:%s", sv.seq, txExecSeq, tx.Payload.TxId)
				return false, s.GetSnapshotSize() + len(s.specialTxTable)
			}
		}

		finalReadKvs[finalKey] = &sv{
			value: txRead.Value,
		}
	}
	finalWriteKvs := make(map[string]*sv, len(txRWSet.TxWrites))
	// Append to write table
	for _, txWrite := range txRWSet.TxWrites {
		finalKey := constructKey(txWrite.ContractName, txWrite.Key)
		finalWriteKvs[finalKey] = &sv{
			value: txWrite.Value,
		}
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	// it is necessary to check sealed secondly
	if s.IsSealed() {
		return false, s.GetSnapshotSize()
	}

	if txExecSeq >= s.GetSnapshotSize() {
		s.applyOptimize(tx, txRWSet, txResult, runVmSuccess, finalReadKvs, finalWriteKvs)
		// 普通交易的snapshot 大小需要加上 specialTxTable，否则可能第一次schedule无法退出
		return true, s.GetSnapshotSize() + len(s.specialTxTable)
	}

	// Double-Check
	// Check whether the dependent state has been modified during the running it
	start := time.Now()
	for finalKey := range finalReadKvs {
		if sv, ok := s.writeTable.getByLock(finalKey); ok {
			if sv.seq >= txExecSeq {
				s.log.Debugf("Key Conflicted %+v-%+v, tx id:%s", sv.seq, txExecSeq, tx.Payload.TxId)
				return false, s.GetSnapshotSize() + len(s.specialTxTable)
			}
		}
	}
	micro := time.Since(start).Microseconds()
	s.applyConflictTime.Add(micro)
	s.applyOptimize(tx, txRWSet, txResult, runVmSuccess, finalReadKvs, finalWriteKvs)
	return true, s.GetSnapshotSize() + len(s.specialTxTable)
}

// 迭代器交易第一次执行，先缓存该笔交易到specialTxTable中，返回true
func (s *SnapshotImpl) applyIteratorFirstRun(tx *commonPb.Transaction) (bool, int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.IsSealed() {
		return false, s.GetSnapshotSize()
	}

	// 迭代器交易且不符合apply special 交易的情况下，往specialTxTable添加该笔交易，且返回true && snapshot的大小以及special交易列表长度
	// 返回true是由于防止将迭代器交易重复执行导致卡死
	s.specialTxTable = append(s.specialTxTable, tx)
	return true, s.GetSnapshotSize() + len(s.specialTxTable)
}

// 迭代器交易第二次执行，正式apply到snapshot中
func (s *SnapshotImpl) applyIteratorSecondRun(tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, runVmSuccess bool,
) (bool, int) {
	txRWSet := txSimContext.GetTxRWSet(runVmSuccess)
	txResult := txSimContext.GetTxResult()

	finalReadKvs, finalWriteKvs := buildKVMaps(txRWSet)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.applyOptimize(tx, txRWSet, txResult, runVmSuccess, finalReadKvs, finalWriteKvs)
	return true, s.GetSnapshotSize()
}

// applyGasTxSimContext apply gas tx to snapshot
func (s *SnapshotImpl) applyGasTxSimContext(tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, runVmSuccess bool,
) (bool, int) {
	txRWSet := txSimContext.GetTxRWSet(runVmSuccess)
	txResult := txSimContext.GetTxResult()

	finalReadKvs, finalWriteKvs := buildKVMaps(txRWSet)

	s.lock.Lock()
	defer s.lock.Unlock()

	// // gas 交易的snapshot 大小已经是完整的了，不需要加 len(s.specialTxTable)
	s.applyOptimize(tx, txRWSet, txResult, runVmSuccess, finalReadKvs, finalWriteKvs)
	return true, s.GetSnapshotSize()
}

func buildKVMaps(txRWSet *commonPb.TxRWSet) (map[string]*sv, map[string]*sv) {
	finalReadKvs := make(map[string]*sv, len(txRWSet.TxReads))
	for _, txRead := range txRWSet.TxReads {
		finalKey := constructKey(txRead.ContractName, txRead.Key)
		finalReadKvs[finalKey] = &sv{value: txRead.Value}
	}

	finalWriteKvs := make(map[string]*sv, len(txRWSet.TxWrites))
	for _, txWrite := range txRWSet.TxWrites {
		finalKey := constructKey(txWrite.ContractName, txWrite.Key)
		finalWriteKvs[finalKey] = &sv{value: txWrite.Value}
	}
	return finalReadKvs, finalWriteKvs
}

// ApplyTxSimContext add TxSimContext to the snapshot, return current applied tx num whether success of not
func (s *SnapshotImpl) ApplyTxSimContext(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
	runVmSuccess bool, applySpecialTx bool,
) (bool, int) {
	tx := txSimContext.GetTx()

	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("apply tx: %s, execOrderTxType:%d, runVmSuccess:%v, applySpecialTx:%v",
			tx.Payload.TxId, specialTxType, runVmSuccess, applySpecialTx)
	})

	switch {
	// 第一次调度执行时，迭代器交易处理逻辑，仅将该笔交易缓存到specialTxTable中
	case !applySpecialTx && specialTxType == protocol.ExecOrderTxTypeSpecial:
		return s.applyIteratorFirstRun(tx)
	// 第二次调度执行时，迭代器交易处理逻辑，正式apply到snapshot
	case applySpecialTx && specialTxType == protocol.ExecOrderTxTypeSpecial:
		return s.applyIteratorSecondRun(tx, txSimContext, runVmSuccess)
	// gas交易处理逻辑，apply gas 交易到snapshot（有gas交易，该逻辑最后执行，否则不执行）
	case specialTxType == protocol.ExecOrderTxTypeChargeGas:
		return s.applyGasTxSimContext(tx, txSimContext, runVmSuccess)
	// 普通交易处理逻辑
	default:
		return s.applyNormalTxSimContext(tx, txSimContext, runVmSuccess)
	}
}

// ApplyBlock apply tx rwset map to block
func (s *SnapshotImpl) ApplyBlock(block *commonPb.Block, txRWSetMap map[string]*commonPb.TxRWSet) {
	if len(block.Txs) != len(txRWSetMap) {
		s.log.Warnf("txs num is: %d, but rwSet num is: %d", len(block.Txs), len(txRWSetMap))
		return
	}
	for _, tx := range block.Txs {
		s.apply(tx, txRWSetMap[tx.Payload.TxId], tx.Result, tx.Result.Code == commonPb.TxStatusCode_SUCCESS)
	}
}

// After the read-write set is generated, add TxSimContext to the snapshot
func (s *SnapshotImpl) apply(tx *commonPb.Transaction, txRWSet *commonPb.TxRWSet, txResult *commonPb.Result,
	runVmSuccess bool,
) {
	// Append to read table
	// applySeq := len(s.txTable)
	applySeq := s.GetSnapshotSize()
	// compatible with version lower than 2201, failed transaction should not apply read set to snapshot
	// that may cause next transaction read out an error value. Failed transaction can produce invalid read set
	// by read, write and then read again the same value.
	if s.blockVersion < 2201 || runVmSuccess {
		for _, txRead := range txRWSet.TxReads {
			finalKey := constructKey(txRead.ContractName, txRead.Key)
			s.readTable.putByLock(finalKey, &sv{
				seq:   applySeq,
				value: txRead.Value,
			})
		}
	}

	// Append to write table
	for _, txWrite := range txRWSet.TxWrites {
		finalKey := constructKey(txWrite.ContractName, txWrite.Key)
		s.writeTable.putByLock(finalKey, &sv{
			seq:   applySeq,
			value: txWrite.Value,
		})
	}

	// Append to read-write-set table
	s.txRWSetTable = append(s.txRWSetTable, txRWSet)
	//s.log.DebugDynamic(func() string {
	//	return fmt.Sprintf("apply tx: %s, rwset.TxReads: %v", tx.Payload.TxId, txRWSet.TxReads)
	//})
	//s.log.DebugDynamic(func() string {
	//	return fmt.Sprintf("apply tx: %s, rwset.TxWrites: %v", tx.Payload.TxId, txRWSet.TxWrites)
	//})
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("apply tx: %s, rwset table size %d", tx.Payload.TxId, len(s.txRWSetTable))
	})

	// Add to tx result map
	s.txResultMap[tx.Payload.TxId] = txResult

	// Add to transaction table
	s.tableLock.Lock()
	defer s.tableLock.Unlock()
	s.txTable = append(s.txTable, tx)
}

// After the read-write set is generated, add TxSimContext to the snapshot
func (s *SnapshotImpl) applyOptimize(tx *commonPb.Transaction, txRWSet *commonPb.TxRWSet, txResult *commonPb.Result,
	runVmSuccess bool, finalReadKvs, finalWriteKvs map[string]*sv,
) {
	// Append to read table
	applySeq := len(s.txTable)
	// applySeq := s.GetSnapshotSize()
	// compatible with version lower than 2201, failed transaction should not apply read set to snapshot
	// that may cause next transaction read out an error value. Failed transaction can produce invalid read set
	// by read, write and then read again the same value.
	var wg sync.WaitGroup
	wg.Add(1)
	if s.blockVersion < 2201 || runVmSuccess {
		wg.Add(1)
		go func() {
			start := time.Now()
			for finalKey, rsv := range finalReadKvs {
				rsv.seq = applySeq
				s.readTable.putByLock(finalKey, rsv)
			}
			wg.Done()
			micro := time.Since(start).Microseconds()
			s.applyAddReadTime.Add(micro)
		}()
	}

	// Append to write table
	go func() {
		start := time.Now()
		for finalKey, wsv := range finalWriteKvs {
			wsv.seq = applySeq
			s.writeTable.putByLock(finalKey, wsv)
		}
		wg.Done()
		micro := time.Since(start).Microseconds()
		s.applyAddWriteTime.Add(micro)
	}()
	wg.Wait()
	// Append to read-write-set table
	s.txRWSetTable = append(s.txRWSetTable, txRWSet)
	//s.log.DebugDynamic(func() string {
	//	return fmt.Sprintf("apply tx: %s, rwset.TxReads: %v", tx.Payload.TxId, txRWSet.TxReads)
	//})
	//s.log.DebugDynamic(func() string {
	//	return fmt.Sprintf("apply tx: %s, rwset.TxWrites: %v", tx.Payload.TxId, txRWSet.TxWrites)
	//})
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("apply tx: %s, rwset table size %d", tx.Payload.TxId, len(s.txRWSetTable))
	})

	// Add to tx result map
	s.txResultMap[tx.Payload.TxId] = txResult

	// Add to transaction table
	s.tableLock.Lock()
	defer s.tableLock.Unlock()
	s.txTable = append(s.txTable, tx)
}

// IsSealed check if snapshot is sealed
func (s *SnapshotImpl) IsSealed() bool {
	return s.sealed.Load()
}

// GetBlockHeight returns current block height
func (s *SnapshotImpl) GetBlockHeight() uint64 {
	return s.blockHeight
}

// GetBlockTimestamp returns current block timestamp
func (s *SnapshotImpl) GetBlockTimestamp() int64 {
	return s.blockTimestamp
}

// GetBlockProposer for current snapshot
func (s *SnapshotImpl) GetBlockProposer() *accesscontrol.Member {
	return s.blockProposer
}

// Seal the snapshot
func (s *SnapshotImpl) Seal() {
	s.sealed.Store(true)
	s.log.Infof("block apply time[%d] is %d, %d, %d", s.blockHeight,
		s.applyConflictTime.Load(), s.applyAddReadTime.Load(), s.applyAddWriteTime.Load())
}

// ApplyWritesToWriteTable 批量应用写集到 writeTable
// 用于确定性调度器Wria直接将一批交易的写集应用到 snapshot，使得后续交易可以读取到这些写入
// 参数：writes - 需要应用的写操作列表
func (s *SnapshotImpl) ApplyWritesToWriteTable(writes []*commonPb.TxWrite) {
	if s.IsSealed() {
		s.log.Warn("Snapshot is sealed, cannot apply writes to writeTable")
		return
	}

	writeCount := len(writes)

	// 使用当前 txTable 大小作为 applySeq
	// 注意：这个 seq 主要用于冲突检测，在确定性调度器中意义不大
	applySeq := len(s.txTable)

	// 如果写入数量较少，串行处理更高效（避免 goroutine 创建开销）
	const concurrentThreshold = 50
	if writeCount < concurrentThreshold {
		for _, write := range writes {
			finalKey := constructKey(write.ContractName, write.Key)
			wsv := &sv{
				seq:   applySeq,
				value: write.Value,
			}
			s.writeTable.putByLock(finalKey, wsv)
		}
		s.log.DebugDynamic(func() string {
			return fmt.Sprintf("Applied %d writes to writeTable with seq=%d (serial)", len(writes), applySeq)
		})
		return
	}

	// 并发处理：将 writes 分成多个批次并发执行
	// 由于 writes 中的 key 都不同（由调度器保证），可以安全并发
	numWorkers := 8                                         // 使用固定数量的 worker，避免创建过多 goroutine
	batchSize := (writeCount + numWorkers - 1) / numWorkers // 向上取整除法

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > writeCount {
			end = writeCount
		}
		if start >= writeCount {
			break
		}

		wg.Add(1)
		go func(batch []*commonPb.TxWrite) {
			defer wg.Done()
			for _, write := range batch {
				finalKey := constructKey(write.ContractName, write.Key)
				wsv := &sv{
					seq:   applySeq,
					value: write.Value,
				}
				s.writeTable.putByLock(finalKey, wsv)
			}
		}(writes[start:end])
	}

	wg.Wait()
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("Applied %d writes to writeTable with seq=%d (concurrent)", len(writes), applySeq)
	})
}

// todo here
//func (s *SnapshotImpl) BuildDAG(isSql bool, txRWSetTable []*commonPb.TxRWSet) *commonPb.DAG {
//	txs := s.GetTxTable()
//	for _, tx := range txs {
//		if _, ok := SZContractList[tx.Payload.ContractName]; !ok {
//			return s.buildNormalDag(isSql, txRWSetTable)
//		}
//	}
//
//	return genDefaultDag(len(txs))
//}
//
//func genDefaultDag(txCount int) *commonPb.DAG {
//	vertexes := make([]*commonPb.DAG_Neighbor, txCount)
//	for i := 0; i < txCount; i++ {
//		vertexes[i] = &commonPb.DAG_Neighbor{
//			Neighbors: make([]uint32, 0, 1),
//		}
//	}
//	return &commonPb.DAG{
//		Vertexes: vertexes,
//	}
//}

// BuildDAG build the block dag according to the read-write table
func (s *SnapshotImpl) BuildDAG(isSql bool, txRWSetTable []*commonPb.TxRWSet) *commonPb.DAG {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var txRWSets []*commonPb.TxRWSet
	if txRWSetTable == nil {
		txRWSets = s.txRWSetTable
	} else {
		txRWSets = txRWSetTable
	}
	txCount := uint32(len(txRWSets))
	s.log.Infof("start to build DAG for block %d with %d txs", s.blockHeight, txCount)
	dag := &commonPb.DAG{}
	if txCount == 0 {
		return dag
	}
	dag.Vertexes = make([]*commonPb.DAG_Neighbor, txCount)

	if isSql {
		for i := uint32(0); i < txCount; i++ {
			dag.Vertexes[i] = &commonPb.DAG_Neighbor{
				Neighbors: make([]uint32, 0, 1),
			}
			if i != 0 {
				dag.Vertexes[i].Neighbors = append(dag.Vertexes[i].Neighbors, uint32(i-1))
			}
		}
		return dag
	}
	// build all txs' readKeyDictionary, writeKeyDictionary, readPos(the pos in readKeyDictionary) and
	// writePos(the pos in writeKeyDictionary)
	readKeyDict, writeKeyDict, readPos, writePos := s.buildDictAndPos(txRWSets)
	reachMap := make([]*bitmap.Bitmap, txCount)
	// build vertexes
	for i := uint32(0); i < txCount; i++ {
		directReachMap := s.buildReachMap(i, txRWSets[i], readKeyDict, writeKeyDict, readPos, writePos, reachMap)
		dag.Vertexes[i] = &commonPb.DAG_Neighbor{
			Neighbors: make([]uint32, 0, 16),
		}
		for _, j := range directReachMap.Pos1() {
			dag.Vertexes[i].Neighbors = append(dag.Vertexes[i].Neighbors, uint32(j))
		}
	}
	s.log.Infof("build DAG for block %d finished", s.blockHeight)
	return dag
}

// buildDictAndPos build read/write key dict and read/write key pos
func (s *SnapshotImpl) buildDictAndPos(txRWSetTable []*commonPb.TxRWSet) (map[string][]uint32, map[string][]uint32,
	map[uint32]map[string]uint32, map[uint32]map[string]uint32,
) {
	// Suppose there are at least 4 keys in each transaction，2 read and 2 write
	readKeyDict := make(map[string][]uint32, len(txRWSetTable)*2)
	writeKeyDict := make(map[string][]uint32, len(txRWSetTable)*2)
	readPos := make(map[uint32]map[string]uint32, len(txRWSetTable))
	writePos := make(map[uint32]map[string]uint32, len(txRWSetTable))
	for i := uint32(0); i < uint32(len(txRWSetTable)); i++ {
		readTableItemForI := txRWSetTable[i].TxReads
		writeTableItemForI := txRWSetTable[i].TxWrites
		readPos[i] = make(map[string]uint32, len(readTableItemForI))
		writePos[i] = make(map[string]uint32, len(writeTableItemForI))
		// put all read key in to readKeyDict and set their pos into readPos and writePos
		for _, keyForI := range readTableItemForI {
			key := string(keyForI.Key)
			readPos[i][key] = uint32(len(readKeyDict[key]))
			writePos[i][key] = uint32(len(writeKeyDict[key]))
			readKeyDict[key] = append(readKeyDict[key], i)
		}
		// put all write key in to writeKeyDict and set their pos into readPos and writePos
		for _, keyForI := range writeTableItemForI {
			key := string(keyForI.Key)
			writePos[i][key] = uint32(len(writeKeyDict[key]))
			_, ok := readPos[i][key]
			if !ok {
				readPos[i][key] = uint32(len(readKeyDict[key]))
			}
			writeKeyDict[key] = append(writeKeyDict[key], i)
		}
	}
	return readKeyDict, writeKeyDict, readPos, writePos
}

func (s *SnapshotImpl) buildReachMap(i uint32, txRWSet *commonPb.TxRWSet, readKeyDict, writeKeyDict map[string][]uint32,
	readPos, writePos map[uint32]map[string]uint32, reachMap []*bitmap.Bitmap,
) *bitmap.Bitmap {
	readTableItemForI := txRWSet.TxReads
	writeTableItemForI := txRWSet.TxWrites
	allReachForI := &bitmap.Bitmap{}
	allReachForI.Set(int(i))
	directReachForI := &bitmap.Bitmap{}

	// ReadSet && WriteSet conflict
	for _, keyForI := range readTableItemForI {
		readKey := string(keyForI.Key)
		writeKeyTxs := writeKeyDict[readKey]
		if len(writeKeyTxs) == 0 {
			continue
		}
		// just check 1 write key before the tx because write keys all are conflict
		j := int(writePos[i][readKey]) - 1
		if j >= 0 && !allReachForI.Has(int(writeKeyTxs[j])) {
			directReachForI.Set(int(writeKeyTxs[j]))
			allReachForI.Or(reachMap[writeKeyTxs[j]])
		}
	}
	// WriteSet and (all ReadSet, WriteSet) conflict
	for _, keyForI := range writeTableItemForI {
		writeKey := string(keyForI.Key)
		readKeyTxs := readKeyDict[writeKey]
		if len(readKeyTxs) > 0 {
			// we should check all readKeyTxs because read keys has no conflict
			j := int(readPos[i][writeKey]) - 1
			for ; j >= 0; j-- {
				if !allReachForI.Has(int(readKeyTxs[j])) {
					directReachForI.Set(int(readKeyTxs[j]))
					allReachForI.Or(reachMap[readKeyTxs[j]])
				}
			}
		}
		writeKeyTxs := writeKeyDict[writeKey]
		if len(writeKeyTxs) == 0 {
			continue
		}
		// just check 1 write key before the tx because write keys all are conflict
		j := int(writePos[i][writeKey]) - 1
		if j >= 0 && !allReachForI.Has(int(writeKeyTxs[j])) {
			directReachForI.Set(int(writeKeyTxs[j]))
			allReachForI.Or(reachMap[writeKeyTxs[j]])
		}
	}
	reachMap[i] = allReachForI
	return directReachForI
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

// SetBlockFingerprint set block fingerprint
func (s *SnapshotImpl) SetBlockFingerprint(fp utils.BlockFingerPrint) {
	s.blockFingerprint = string(fp)
}

// GetBlockFingerprint returns current block fingerprint
func (s *SnapshotImpl) GetBlockFingerprint() string {
	return s.blockFingerprint
}

func (s *SnapshotImpl) dealNormalTx(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
	runVmSuccess, applySpecialTx bool, tx *commonPb.Transaction,
) (bool, int) {
	s.log.Infof("what??? deal normal Tx, contractName: %s", tx.Payload.ContractName)
	s.lock.Lock()
	defer s.lock.Unlock()
	// it is necessary to check sealed secondly
	if !applySpecialTx && s.IsSealed() {
		return false, len(s.txTable)
	}

	txExecSeq := txSimContext.GetTxExecSeq()
	var txRWSet *commonPb.TxRWSet
	var txResult *commonPb.Result

	if !applySpecialTx && specialTxType == protocol.ExecOrderTxTypeSpecial {
		s.specialTxTable = append(s.specialTxTable, tx)
		return true, len(s.txTable) + len(s.specialTxTable)
	}

	// Only when the virtual machine is running normally can the read-write set be saved, or write fake conflicted key
	txRWSet = txSimContext.GetTxRWSet(runVmSuccess)
	txResult = txSimContext.GetTxResult()

	if specialTxType == protocol.ExecOrderTxTypeSpecial || txExecSeq >= len(s.txTable) {
		s.apply(tx, txRWSet, txResult, runVmSuccess)
		return true, len(s.txTable)
	}

	// Check whether the dependent state has been modified during the running it
	for _, txRead := range txRWSet.TxReads {
		finalKey := constructKey(txRead.ContractName, txRead.Key)
		if sv, ok := s.writeTable.getByLock(finalKey); ok {
			if sv.seq >= txExecSeq {
				s.log.Debugf("Key Conflicted %+v-%+v, tx id:%s", sv.seq, txExecSeq, tx.Payload.TxId)
				return false, len(s.txTable)
			}
		}
	}

	s.apply(tx, txRWSet, txResult, runVmSuccess)
	return true, len(s.txTable)
}

func (s *SnapshotImpl) dealSZTx(txSimContext protocol.TxSimContext, specialTxType protocol.ExecOrderTxType,
	runVmSuccess, applySpecialTx bool, tx *commonPb.Transaction,
) (bool, int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	// it is necessary to check sealed secondly
	if !applySpecialTx && s.IsSealed() {
		return false, len(s.txTable)
	}

	txExecSeq := txSimContext.GetTxExecSeq()
	var txRWSet *commonPb.TxRWSet
	var txResult *commonPb.Result

	//txRWSet = &commonPb.TxRWSet{
	//	TxId:     txSimContext.GetTx().Payload.TxId,
	//	TxReads:  []*commonPb.TxRead{},
	//	TxWrites: []*commonPb.TxWrite{},
	//}
	if !applySpecialTx && specialTxType == protocol.ExecOrderTxTypeSpecial {
		s.specialTxTable = append(s.specialTxTable, tx)
		return true, len(s.txTable) + len(s.specialTxTable)
	}

	// Only when the virtual machine is running normally can the read-write set be saved, or write fake conflicted key
	// TODO disable getting read/write sets from txSimContext
	txRWSet = txSimContext.GetTxRWSet(runVmSuccess)
	txResult = txSimContext.GetTxResult()

	if specialTxType == protocol.ExecOrderTxTypeSpecial || txExecSeq >= len(s.txTable) {
		s.apply(tx, txRWSet, txResult, runVmSuccess)
		return true, len(s.txTable)
	}

	// Check whether the dependent state has been modified during the running it
	for _, txRead := range txRWSet.TxReads {
		finalKey := constructKey(txRead.ContractName, txRead.Key)
		if sv, ok := s.writeTable.getByLock(finalKey); ok {
			if sv.seq >= txExecSeq {
				s.log.Debugf("Key Conflicted %+v-%+v, tx id:%s", sv.seq, txExecSeq, tx.Payload.TxId)
				return false, len(s.txTable)
			}
		}
	}

	s.apply(tx, txRWSet, txResult, runVmSuccess)
	return true, len(s.txTable)
}
