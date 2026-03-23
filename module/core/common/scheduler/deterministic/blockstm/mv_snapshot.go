package blockstm

import (
	"fmt"

	vmPb "chainmaker.org/chainmaker/pb-go/v2/vm"
	"chainmaker.org/chainmaker/protocol/v2"
)

// MVReadBlockedError indicates the read should be retried after BlockingTxnIdx finishes.
// It corresponds to Block-STM READ_ERROR when observing an ESTIMATE in MVMemory.
type MVReadBlockedError struct {
	BlockingTxnIdx TxnIndex
}

func (e *MVReadBlockedError) Error() string {
	return fmt.Sprintf("mv memory read blocked by txn %d", e.BlockingTxnIdx)
}

func (e *MVReadBlockedError) BlockingTxnIndex() int {
	return int(e.BlockingTxnIdx)
}

type mvSnapshot struct {
	protocol.Snapshot

	mv     *MVMemory
	txnIdx TxnIndex

	reads map[Key]TxnVersion
}

func newMVSnapshot(base protocol.Snapshot, mv *MVMemory, txnIdx TxnIndex) *mvSnapshot {
	return &mvSnapshot{
		Snapshot: base,
		mv:       mv,
		txnIdx:   txnIdx,
		reads:    make(map[Key]TxnVersion, 64),
	}
}

// GetSnapshotSize is used by vm.NewTxSimContext() to set txExecSeq.
// For Block-STM, txExecSeq must be the transaction index.
func (s *mvSnapshot) GetSnapshotSize() int {
	return int(s.txnIdx)
}

func constructKey(contractName string, key []byte) Key {
	// Match chainmaker's current fast key construction.
	// Note: this may cause ambiguities; caller acknowledged and accepted for now.
	return Key(contractName + string(key))
}

func (s *mvSnapshot) recordRead(loc Key, version TxnVersion) {
	s.reads[loc] = version
}

func (s *mvSnapshot) ReadSet() ReadSet {
	reads := make([]ReadDescriptor, 0, len(s.reads))
	for k, v := range s.reads {
		reads = append(reads, ReadDescriptor{
			Key:     k,
			Version: v,
		})
	}
	return ReadSet{Reads: reads}
}

// todo：可以清楚下什么时候是snapshot.GetKey/GetKeyWithLock/GetKeys/GetKeysWithLock，我估计都是调用GetKey
func (s *mvSnapshot) GetKey(txExecSeq int, contractName string, key []byte) ([]byte, error) {
	loc := constructKey(contractName, key)

	rr := s.mv.read(loc, TxnIndex(txExecSeq))
	switch rr.Status {
	case ReadStatusNotFound:
		s.recordRead(loc, InvalidTxnVersion)                      // ⊥ InvalidTxnVersion 表示从db中读取的
		v, err := s.Snapshot.GetKey(txExecSeq, contractName, key) // 从db中读
		if err != nil {
			return nil, err
		}
		return v, nil
	case ReadStatusOK:
		s.recordRead(loc, rr.Version)
		return rr.Value, nil
	case ReadStatusError:
		return nil, &MVReadBlockedError{BlockingTxnIdx: rr.BlockingTxnIdx} // todo：读到ERROR
	default:
		return nil, fmt.Errorf("unknown mv read status: %v", rr.Status)
	}
}

func (s *mvSnapshot) GetKeyWithLock(txExecSeq int, contractName, lockerName, txId string, key []byte) ([]byte, error) {
	loc := constructKey(contractName, key)

	rr := s.mv.read(loc, TxnIndex(txExecSeq))
	switch rr.Status {
	case ReadStatusOK:
		s.recordRead(loc, rr.Version)
		return rr.Value, nil
	case ReadStatusNotFound:
		v, err := s.Snapshot.GetKeyWithLock(txExecSeq, contractName, lockerName, txId, key)
		if err != nil {
			return nil, err
		}
		s.recordRead(loc, InvalidTxnVersion) // ⊥
		return v, nil
	case ReadStatusError:
		return nil, &MVReadBlockedError{BlockingTxnIdx: rr.BlockingTxnIdx}
	default:
		return nil, fmt.Errorf("unknown mv read status: %v", rr.Status)
	}
}

func (s *mvSnapshot) GetKeys(txExecSeq int, keys []*vmPb.BatchKey) ([]*vmPb.BatchKey, error) {
	// Correctness-first implementation: read each key with MVMemory preference.
	// This preserves the invariant that any snapshot read observes MVMemory first.
	for _, k := range keys {
		stateKey := protocol.GetKeyStr(k.Key, k.Field)
		v, err := s.GetKey(txExecSeq, k.ContractName, stateKey)
		if err != nil {
			return nil, err
		}
		k.Value = v
	}
	return keys, nil
}

func (s *mvSnapshot) GetKeysWithLock(txExecSeq int, keys []*vmPb.BatchKey, lockerName, txId string) ([]*vmPb.BatchKey, error) {
	for _, k := range keys {
		stateKey := protocol.GetKeyStr(k.Key, k.Field)
		v, err := s.GetKeyWithLock(txExecSeq, k.ContractName, lockerName, txId, stateKey)
		if err != nil {
			return nil, err
		}
		k.Value = v
	}
	return keys, nil
}
