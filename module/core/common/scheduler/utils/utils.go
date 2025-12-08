/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/hokaccha/go-prettyjson"

	"chainmaker.org/chainmaker-go/module/accesscontrol"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/localconf/v2"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
)

// InitSigner init a signer with node private key
func InitSigner(
	chainConfig *configPb.ChainConfig,
	cmConfig *localconf.CMConfig) (protocol.SigningMember, error) {
	var err error
	var signingMember protocol.SigningMember
	nodeConfig := cmConfig.NodeConfig

	switch chainConfig.AuthType {
	case protocol.PermissionedWithCert, protocol.Identity:
		signingMember, err = accesscontrol.InitCertSigningMember(
			chainConfig,
			nodeConfig.OrgId,
			nodeConfig.PrivKeyFile,
			nodeConfig.PrivKeyPassword,
			nodeConfig.CertFile)
		if err != nil {
			return nil, fmt.Errorf("InitCertSigningMember failed: err = %v", err)
		}
	case protocol.PermissionedWithKey, protocol.Public:
		signingMember, err = accesscontrol.InitPKSigningMember(
			chainConfig.Crypto.Hash,
			nodeConfig.OrgId,
			nodeConfig.PrivKeyFile,
			nodeConfig.PrivKeyPassword)
		if err != nil {
			return nil, fmt.Errorf("InitPKSigningMember failed: err = %v", err)
		}
	default:
		return nil, fmt.Errorf("unknown auth type: %v", chainConfig.AuthType)
	}

	return signingMember, nil
}

func IsOptimizeChargeGasEnabled(chainConf protocol.ChainConf) bool {
	enableGas := false
	enableOptimizeChargeGas := false
	if chainConf.ChainConfig() != nil && chainConf.ChainConfig().AccountConfig != nil {
		enableGas = chainConf.ChainConfig().AccountConfig.EnableGas
	}

	if chainConf.ChainConfig() != nil && chainConf.ChainConfig().Core != nil {
		enableOptimizeChargeGas = chainConf.ChainConfig().Core.EnableOptimizeChargeGas
	}
	return enableGas && enableOptimizeChargeGas
}

func GetPkFromTx(tx *commonPb.Transaction, snapshot protocol.Snapshot) (crypto.PublicKey, error) {

	var err error
	var pk []byte
	var publicKey crypto.PublicKey
	signingMember := tx.GetSender().GetSigner()
	if signingMember == nil {
		err = errors.New(" can not find sender from tx ")
		return nil, err
	}

	switch signingMember.MemberType {
	case acPb.MemberType_CERT:
		pk, err = publicKeyFromCert(signingMember.MemberInfo)
		if err != nil {
			return nil, err
		}
		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return nil, err
		}

	case acPb.MemberType_CERT_HASH:
		var certInfo *commonPb.CertInfo
		infoHex := hex.EncodeToString(signingMember.MemberInfo)
		if certInfo, err = wholeCertInfoFromSnapshot(snapshot, infoHex); err != nil {
			return nil, fmt.Errorf(" can not load the whole cert info,member[%s],reason: %s", infoHex, err)
		}

		pk, err = publicKeyFromCert(certInfo.Cert)
		if err != nil {
			return nil, err
		}

		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return nil, err
		}

	case acPb.MemberType_PUBLIC_KEY:
		pk = signingMember.MemberInfo
		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return nil, err
		}

	default:
		err = fmt.Errorf("invalid member type: %s", signingMember.MemberType)
		return nil, err
	}

	return publicKey, nil
}

// parseUserAddress
func publicKeyFromCert(member []byte) ([]byte, error) {
	certificate, err := utils.ParseCert(member)
	if err != nil {
		return nil, err
	}
	pubKeyStr, err := certificate.PublicKey.String()
	if err != nil {
		return nil, err
	}
	return []byte(pubKeyStr), nil
}

// PublicKeyToAddress generate address from public key, according to chainconfig parameter
func PublicKeyToAddress(pk crypto.PublicKey, chainCfg *configPb.ChainConfig) (string, error) {

	publicKeyString, err := utils.PkToAddrStr(pk, chainCfg.Vm.AddrType, crypto.HashAlgoMap[chainCfg.Crypto.Hash])
	if err != nil {
		return "", err
	}

	if chainCfg.Vm.AddrType == configPb.AddrType_ZXL {
		publicKeyString = "ZX" + publicKeyString
	}
	return publicKeyString, nil
}

func wholeCertInfoFromSnapshot(snapshot protocol.Snapshot, certHash string) (*commonPb.CertInfo, error) {
	certBytes, err := snapshot.GetKey(-1, syscontract.SystemContract_CERT_MANAGE.String(), []byte(certHash))
	if err != nil {
		return nil, err
	}

	return &commonPb.CertInfo{
		Hash: certHash,
		Cert: certBytes,
	}, nil
}

// ConstructKey return key
func ConstructKey(contractName string, key []byte) string {
	return contractName + string(key)
}

func getTxSenderSigner(tx *commonPb.Transaction) *acPb.Member {
	return tx.GetSender().GetSigner()
}

func getTxPayerSigner(tx *commonPb.Transaction) *acPb.Member {
	payer := tx.GetPayer()
	if payer == nil {
		return nil
	}

	return payer.GetSigner()
}

func publicKeyPEMFromMember(member *acPb.Member, snapshot protocol.Snapshot) ([]byte, error) {

	var pk []byte
	var err error

	switch member.MemberType {
	case acPb.MemberType_CERT:
		pk, err = publicKeyFromCert(member.MemberInfo)
		if err != nil {
			return nil, err
		}

	case acPb.MemberType_CERT_HASH:
		var certInfo *commonPb.CertInfo
		infoHex := hex.EncodeToString(member.MemberInfo)
		if certInfo, err = wholeCertInfoFromSnapshot(snapshot, infoHex); err != nil {
			return nil, fmt.Errorf(" can not load the whole cert info,member[%s],reason: %s", infoHex, err)
		}

		pk, err = publicKeyFromCert(certInfo.Cert)
		if err != nil {
			return nil, err
		}

	case acPb.MemberType_PUBLIC_KEY:
		pk = member.MemberInfo

	default:
		err = fmt.Errorf("invalid member type: %s", member.MemberType)
		return nil, err
	}

	return pk, nil
}

func getPayerFromContract(tx *commonPb.Transaction, snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider) ([]byte, error) {
	contractName := tx.GetPayload().GetContractName()
	method := tx.GetPayload().GetMethod()

	var pkBytes []byte
	var err error

	// 先从缓存查
	_, pkBytes, _ = utils.GetContractMethodPayerPKFromAC(ac, contractName, method)
	if pkBytes != nil {
		return pkBytes, nil
	}

	// 缓存查不到从snapshot查
	key, value, err := utils.GetContractMethodPayerPK(snapshot, contractName, method)
	if err != nil {
		return nil, fmt.Errorf("get contract method payer failed, error: %v", err)
	}
	// 加入缓存
	if value != nil {
		_ = ac.SetPayerToCache(key, value)
	}
	return value, nil
}

// todo: merge with getPayerPk
func GetPayerPkFromTx(
	tx *commonPb.Transaction,
	snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider) ([]byte, error) {

	var err error
	var publicKeyPEM []byte

	// 首先，检查 tx payer
	member := getTxPayerSigner(tx)
	if member != nil {
		publicKeyPEM, err = publicKeyPEMFromMember(member, snapshot)
		if err != nil {
			return nil, err
		}
		return publicKeyPEM, nil
	}

	// 其次，检查合约设置
	publicKeyPEM, err = getPayerFromContract(tx, snapshot, ac)
	if err != nil {
		return nil, fmt.Errorf("get contract method payer failed, err = %v", err)
	}
	if publicKeyPEM != nil {
		return publicKeyPEM, nil
	}

	// 最后, 检查 sender
	member = getTxSenderSigner(tx)
	if member != nil {
		publicKeyPEM, err = publicKeyPEMFromMember(member, snapshot)
		if err != nil {
			return nil, err
		}
		return publicKeyPEM, nil
	}

	return publicKeyPEM, nil
}

func GetPayerAddressAndPkFromTx(tx *commonPb.Transaction,
	snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider) (string, crypto.PublicKey, error) {

	var (
		pk         crypto.PublicKey
		pkBytes    []byte
		err        error
		addressStr string
	)

	pkBytes, err = GetPayerPkFromTx(tx, snapshot, ac)
	if err != nil {
		return "", nil, fmt.Errorf("GetPayerPkFromTx error: %v", err)
	}

	addressStr, pk, err = ac.GetAddressFromCache(pkBytes)
	if err != nil {
		return "", pk, fmt.Errorf("GetAddressFromCache failed: err = %v", err)
	}

	return addressStr, pk, nil

}

// GetTxRWSetTable from snapshot get TxRWSet map
func GetTxRWSetTable(snapshot protocol.Snapshot, block *commonPb.Block, log protocol.Logger) map[string]*commonPb.TxRWSet {
	txRWSetMap := make(map[string]*commonPb.TxRWSet)
	txRWSetTable := snapshot.GetTxRWSetTable()
	for _, txRWSet := range txRWSetTable {
		if txRWSet != nil {
			txRWSetMap[txRWSet.TxId] = txRWSet
		}
	}
	//ts.dumpDAG(block.Dag, block.Txs)
	if localconf.ChainMakerConfig.SchedulerConfig.RWSetLog {
		result, _ := prettyjson.Marshal(txRWSetMap)
		log.Infof("schedule rwset :%s, dag:%+v", result, block.Dag)
	}
	return txRWSetMap
}

// GetContractEventMap from block tx result get ContractEvent map
func GetContractEventMap(block *commonPb.Block) map[string][]*commonPb.ContractEvent {
	contractEventMap := make(map[string][]*commonPb.ContractEvent)
	for _, tx := range block.Txs {
		event := tx.Result.ContractResult.ContractEvent
		contractEventMap[tx.Payload.TxId] = event
	}
	return contractEventMap
}
