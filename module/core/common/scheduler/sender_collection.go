package scheduler

import (
	"fmt"

	"chainmaker.org/chainmaker/common/v2/crypto"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// SenderCollection contains:
// key: address
// value: tx collection will address's other data
type SenderCollection struct {
	txsMap map[string]*TxCollection
}

type TxCollection struct {
	// public key to generate address
	publicKey crypto.PublicKey
	// balance of the address saved at SenderCollection
	accountBalance int64
	// total gas added each tx
	totalGasUsed int64
	txs          []*commonPb.Transaction
}

func (g *TxCollection) String() string {
	pubKeyStr, _ := g.publicKey.String()
	return fmt.Sprintf(
		"\nTxsGroup{ \n\tpublicKey: %s, \n\taccountBalance: %v, \n\ttotalGasUsed: %v, \n\ttxs: [%d items] }",
		pubKeyStr, g.accountBalance, g.totalGasUsed, len(g.txs))
}

func NewSenderCollection(
	txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
	log protocol.Logger) *SenderCollection {
	return &SenderCollection{
		txsMap: getSenderTxCollection(txBatch, snapshot, log),
	}
}

// getSenderTxCollection split txs in txBatch by sender account
func getSenderTxCollection(
	txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
	log protocol.Logger) map[string]*TxCollection {
	txCollectionMap := make(map[string]*TxCollection)

	var err error
	chainCfg := snapshot.GetLastChainConfig()

	for _, tx := range txBatch {
		// get the public key from tx
		pk, err2 := getPkFromTx(tx, snapshot)
		if err2 != nil {
			log.Errorf("getPkFromTx failed: err = %v", err2)
			continue
		}

		// convert the public key to `ZX` or `CM` or `EVM` address
		address, err2 := publicKeyToAddress(pk, chainCfg)
		if err2 != nil {
			log.Error("publicKeyToAddress failed: err = %v", err2)
			continue
		}

		txCollection, exists := txCollectionMap[address]
		if !exists {
			txCollection = &TxCollection{
				publicKey:      pk,
				accountBalance: int64(0),
				totalGasUsed:   int64(0),
				txs:            make([]*commonPb.Transaction, 0),
			}
			txCollectionMap[address] = txCollection
		}
		txCollection.txs = append(txCollection.txs, tx)
	}

	for senderAddress, txCollection := range txCollectionMap {
		// get the account balance from snapshot
		txCollection.accountBalance, err = getAccountBalanceFromSnapshot(senderAddress, snapshot)
		if err != nil {
			errMsg := fmt.Sprintf("get account balance failed: err = %v", err)
			log.Error(errMsg)
			for _, tx := range txCollection.txs {
				tx.Result = &commonPb.Result{
					Code: commonPb.TxStatusCode_CONTRACT_FAIL,
					ContractResult: &commonPb.ContractResult{
						Code:    uint32(1),
						Result:  nil,
						Message: errMsg,
						GasUsed: uint64(0),
					},
					RwSetHash: nil,
					Message:   errMsg,
				}
			}
		}
	}

	return txCollectionMap
}

func (s SenderCollection) Clear() {
	for addr := range s.txsMap {
		delete(s.txsMap, addr)
	}
}
