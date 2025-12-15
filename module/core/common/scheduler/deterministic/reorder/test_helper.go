/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package reorder

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	crypto2 "chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/utils/v2"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"

	"github.com/golang/mock/gomock"
)

var (
	TestPrivKeyFile = "../../../../../config/wx-org1/certs/node/consensus1/consensus1.sign.key"
	TestCertFile    = "../../../../../config/wx-org1/certs/node/consensus1/consensus1.sign.crt"
)

// newTx creates a test transaction
func newTx(txId string, contractId *commonPb.Contract, parameterMap map[string]string) *commonPb.Transaction {
	var parameters []*commonPb.KeyValuePair
	for key, value := range parameterMap {
		parameters = append(parameters, &commonPb.KeyValuePair{
			Key:   key,
			Value: []byte(value),
		})
	}

	return &commonPb.Transaction{
		Payload: &commonPb.Payload{
			ChainId:        "Chain1",
			TxType:         0,
			TxId:           txId,
			ContractName:   contractId.Name,
			Method:         "method",
			Parameters:     parameters,
			Timestamp:      0,
			ExpirationTime: 0,
			Limit:          &commonPb.Limit{GasLimit: 0},
		},
		Result: &commonPb.Result{
			Code: commonPb.TxStatusCode_SUCCESS,
			ContractResult: &commonPb.ContractResult{
				Code:          0,
				Result:        nil,
				Message:       "",
				GasUsed:       0,
				ContractEvent: nil,
			},
			RwSetHash: nil,
		},
		Sender: &commonPb.EndorsementEntry{Signer: &acPb.Member{OrgId: "org1", MemberInfo: []byte("-----BEGIN CERTIFICATE-----\nMIICdDCCAhqgAwIBAgIDDqVbMAoGCCqGSM49BAMCMIGIMQswCQYDVQQGEwJDTjEQ\nMA4GA1UECBMHQmVpamluZzEQMA4GA1UEBxMHQmVpamluZzEeMBwGA1UEChMVd3gt\nb3JnLmNoYWlubWFrZXIub3JnMRIwEAYDVQQLEwlyb290LWNlcnQxITAfBgNVBAMT\nGGNhLnd4LW9yZy5jaGFpbm1ha2VyLm9yZzAeFw0yMjA4MjMwMzExNTlaFw0yNzA4\nMjIwMzExNTlaMIGPMQswCQYDVQQGEwJDTjEQMA4GA1UECBMHQmVpamluZzEQMA4G\nA1UEBxMHQmVpamluZzEeMBwGA1UEChMVd3gtb3JnLmNoYWlubWFrZXIub3JnMQ8w\nDQYDVQQLEwZjbGllbnQxKzApBgNVBAMTImNsaWVudDEuc2lnbi53eC1vcmcuY2hh\naW5tYWtlci5vcmcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASk26B+moZKChBk\nhoZ35x21a9YkRGldmVn32pDPSBKYgML57B7aJke0Sh38ULVZUfHMsYin4sYBbyY9\njqQaIR5bo2owaDAOBgNVHQ8BAf8EBAMCBsAwKQYDVR0OBCIEIOYu3/LFYp+U2FaF\nQehWEd9wYY4zx4hiDb/YhYysSA+7MCsGA1UdIwQkMCKAIEKT6aAzVPy7Fhu6s1GJ\nWFp2pZRwxoHdLivN2/18vPckMAoGCCqGSM49BAMCA0gAMEUCIQDOFsdUTe9XQ8iz\nfEjdeQGYyS80PCFnobU70bfFbGu9bQIgNLcZhs1nG6RM/bEdwDQeSSJExBK9NkuC\nch16zyC5Krk=\n-----END CERTIFICATE-----")},
			Signature: []byte("sign1"),
		},
	}
}

// newBlock creates a test block
func newBlock() *commonPb.Block {
	return &commonPb.Block{
		Header: &commonPb.BlockHeader{
			ChainId:        "",
			BlockHeight:    10,
			PreBlockHash:   nil,
			BlockHash:      nil,
			BlockVersion:   2300,
			DagHash:        nil,
			RwSetRoot:      nil,
			TxRoot:         nil,
			BlockTimestamp: 0,
			Proposer:       nil,
			ConsensusArgs:  nil,
			TxCount:        0,
			Signature:      nil,
		},
		Dag: &commonPb.DAG{
			Vertexes: nil,
		},
		Txs: nil,
		AdditionalData: &commonPb.AdditionalData{
			ExtraData: nil,
		},
	}
}

// initAC initializes access control provider mock
func initAC(ctl *gomock.Controller) protocol.AccessControlProvider {
	ac := mock.NewMockAccessControlProvider(ctl)
	ac.EXPECT().GetAddressFromCache(gomock.Any()).DoAndReturn(func(pkBytes []byte) (string, crypto2.PublicKey, error) {
		pk, err := asym.PublicKeyFromPEM(pkBytes)
		if err != nil {
			return "", nil, fmt.Errorf("new public key member failed: parse the public key from PEM failed")
		}

		publicKeyString, err := utils.PkToAddrStr(pk, configpb.AddrType_ZXL, crypto2.HASH_TYPE_SHA256)
		if err != nil {
			return "", nil, err
		}

		publicKeyString = "ZX" + publicKeyString

		return publicKeyString, pk, nil
	}).AnyTimes()
	ac.EXPECT().GetPayerFromCache(gomock.Any()).DoAndReturn(func(key []byte) ([]byte, error) {
		return nil, nil
	}).AnyTimes()
	ac.EXPECT().SetPayerToCache(gomock.Any(), gomock.Any()).DoAndReturn(func(key []byte, value []byte) error {
		return nil
	}).AnyTimes()
	return ac
}

// prepare prepares test environment including mocks and test data
func prepare(t *testing.T, enableSenderGroup, enableConflictsBitWindow bool, txCount int, setVM bool) (
	*mock.MockVmManager, []*commonPb.TxRWSet, []*commonPb.Transaction,
	*mock.MockSnapshot, protocol.TxScheduler, *commonPb.Contract, *commonPb.Block) {

	var txRWSetTable = make([]*commonPb.TxRWSet, txCount)
	for i := 0; i < txCount; i++ {
		txRWSetTable[i] = &commonPb.TxRWSet{TxId: fmt.Sprintf("a000000000000000000000000000%04d", i)}
	}
	var txTable = make([]*commonPb.Transaction, txCount)

	ctl := gomock.NewController(t)
	ac := initAC(ctl)
	snapshot := mock.NewMockSnapshot(ctl)
	vmMgr := mock.NewMockVmManager(ctl)
	vmMgr.EXPECT().BeforeSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()
	vmMgr.EXPECT().AfterSchedule(gomock.Any(), gomock.Any()).Return().AnyTimes()
	chainConf := mock.NewMockChainConf(ctl)
	ledgerCache := mock.NewMockLedgerCache(ctl)

	crypto := configpb.CryptoConfig{
		Hash: crypto2.CRYPTO_ALGO_SHA256,
	}
	contractConf := configpb.ContractConfig{EnableSqlSupport: false}
	chainConfig := &configpb.ChainConfig{
		Crypto:   &crypto,
		Contract: &contractConf,
		Core: &configpb.CoreConfig{
			EnableSenderGroup:        enableSenderGroup,
			EnableConflictsBitWindow: enableConflictsBitWindow,
		},
		AuthType: protocol.Identity,
		Vm: &configpb.Vm{
			AddrType: configpb.AddrType_CHAINMAKER,
		},
		Consensus: &configpb.ConsensusConfig{Type: consensus.ConsensusType_TBFT},
	}
	chainConf.EXPECT().ChainConfig().Return(chainConfig).AnyTimes()

	storeHelper := mock.NewMockStoreHelper(ctl)
	storeHelper.EXPECT().GetPoolCapacity().Return(runtime.NumCPU() * 4).AnyTimes()

	// 创建 reorder scheduler
	scheduler := NewReorderTxScheduler(vmMgr, chainConf, storeHelper, ac)

	contractId := &commonPb.Contract{
		Name:        "ContractName",
		Version:     "1",
		RuntimeType: commonPb.RuntimeType_WASMER,
	}

	contractResult := &commonPb.ContractResult{
		Code:    0,
		Result:  nil,
		Message: "",
	}
	block := newBlock()

	snapshot.EXPECT().GetTxTable().AnyTimes().Return(txTable)
	snapshot.EXPECT().GetTxRWSetTable().AnyTimes().Return(txRWSetTable)
	snapshot.EXPECT().GetSnapshotSize().AnyTimes().Return(len(txTable))
	snapshot.EXPECT().GetSpecialTxTable().AnyTimes().Return([]*commonPb.Transaction{})
	snapshot.EXPECT().GetBlockFingerprint().AnyTimes().Return(strconv.FormatUint(block.Header.BlockHeight, 10))
	snapshot.EXPECT().GetLastChainConfig().Return(chainConfig).AnyTimes()
	snapshot.EXPECT().GetTxResultMap().Return(make(map[string]*commonPb.Result)).AnyTimes()
	blockChainStore := mock.NewMockBlockchainStore(ctl)
	blockChainStore.EXPECT().GetContractByName(contractId.Name).Return(contractId, nil).AnyTimes()
	blockChainStore.EXPECT().GetContractBytecode(contractId.Name).AnyTimes()
	ledgerCache.EXPECT().CurrentHeight().Return(block.Header.BlockHeight-1, nil).AnyTimes()

	snapshot.EXPECT().GetBlockchainStore().AnyTimes().Return(blockChainStore)

	if setVM {
		vmMgr.EXPECT().RunContract(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(contractResult, protocol.ExecOrderTxTypeNormal, commonPb.TxStatusCode_SUCCESS)
	}

	return vmMgr, txRWSetTable, txTable, snapshot, scheduler, contractId, block
}
