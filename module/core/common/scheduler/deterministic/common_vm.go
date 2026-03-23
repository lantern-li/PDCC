/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deterministic

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/sync/singleflight"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"chainmaker.org/chainmaker/vm-native/v2/accountmgr"
	"chainmaker.org/chainmaker/vm/v2"

	"chainmaker.org/chainmaker-go/module/core/common/coinbasemgr"
	schedulerUtils "chainmaker.org/chainmaker-go/module/core/common/scheduler/utils"
)

const (
	// ErrMsgOfGasLimitNotSet error message when gas limit is not set
	ErrMsgOfGasLimitNotSet = "field `GasLimit` must be set in payload."
	blockVersion2300       = uint32(2300)
)

var (
	// sf is a singleflight group to prevent cache stampede when fetching contracts
	sf singleflight.Group
)

type blockSTMReadBlockedError struct {
	blockingTxnIdx int
}

func (e *blockSTMReadBlockedError) Error() string {
	return fmt.Sprintf("blockstm read blocked by txn %d", e.blockingTxnIdx)
}

func (e *blockSTMReadBlockedError) BlockingTxnIndex() int {
	return e.blockingTxnIdx
}

func parseBlockSTMReadBlocked(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	const prefix = "BLOCKSTM_READ_BLOCKED:"
	msg := err.Error()
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return 0, false
	}
	start := idx + len(prefix)
	end := start
	for end < len(msg) {
		ch := msg[end]
		if ch < '0' || ch > '9' {
			break
		}
		end++
	}
	if end == start {
		return 0, false
	}
	n, convErr := strconv.Atoi(msg[start:end])
	if convErr != nil {
		return 0, false
	}
	return n, true
}

// CommonVMHelper contains shared VM execution logic
type CommonVMHelper struct {
	log           protocol.Logger
	chainConf     protocol.ChainConf
	keyReg        *regexp.Regexp
	vmManager     protocol.VmManager
	ac            protocol.AccessControlProvider
	contractCache *sync.Map
}

// NewCommonVMHelper creates a new CommonVMHelper
func NewCommonVMHelper(log protocol.Logger, chainConf protocol.ChainConf, vmManager protocol.VmManager, ac protocol.AccessControlProvider) *CommonVMHelper {
	var commonVMHelper = &CommonVMHelper{
		log:           log,
		chainConf:     chainConf,
		keyReg:        nil,
		vmManager:     vmManager,
		ac:            ac,
		contractCache: &sync.Map{},
	}
	var err error
	commonVMHelper.keyReg, err = regexp.Compile(protocol.DefaultStateRegex)
	if err != nil {
		log.Fatalf("compile default state regex error: %v", err)
		return nil
	}

	return commonVMHelper

}

// errResult creates an error result
func errResult(result *commonPb.Result, err error) (*commonPb.Result, protocol.ExecOrderTxType, error) {
	result.ContractResult.Message = err.Error()
	result.Code = commonPb.TxStatusCode_INVALID_PARAMETER
	result.ContractResult.Code = 1
	return result, protocol.ExecOrderTxTypeNormal, err
}

// CheckGasEnable checks if gas is enabled in chain config
func (h *CommonVMHelper) CheckGasEnable() bool {
	if h.chainConf.ChainConfig() != nil && h.chainConf.ChainConfig().AccountConfig != nil {
		h.log.Debugf("chain config account config enable gas is:%v",
			h.chainConf.ChainConfig().AccountConfig.EnableGas)
		return h.chainConf.ChainConfig().AccountConfig.EnableGas
	}
	return false
}

// CheckNativeFilter checks if the contract/method needs gas charging
func (h *CommonVMHelper) CheckNativeFilter(contractName, method string,
	tx *commonPb.Transaction, snapshot protocol.Snapshot) bool {

	h.log.Debugf("checkNativeFilter => contractName = %s, method = %s", contractName, method)
	if !utils.IsNativeContract(contractName) {
		return true
	}
	if method == syscontract.ContractManageFunction_INIT_CONTRACT.String() ||
		method == syscontract.ContractManageFunction_UPGRADE_CONTRACT.String() {
		return true
	}

	// Handle multi-sign contract special case
	if contractName == syscontract.SystemContract_MULTI_SIGN.String() &&
		method == syscontract.MultiSignFunction_TRIG.String() {
		if getMultiSignEnableManualRun(h.chainConf.ChainConfig()) {
			var multiSignReqId []byte
			for _, kvpair := range tx.Payload.Parameters {
				if kvpair.Key == syscontract.MultiVote_TX_ID.String() {
					multiSignReqId = kvpair.Value
				}
			}
			multiSignInfoBytes, err := snapshot.GetKey(-1, contractName, multiSignReqId)
			if err != nil {
				h.log.Errorf("read multi-sign failed, multiSignReqId = %v, err = %v", multiSignReqId, err)
				return true
			}

			multiSignInfo := &syscontract.MultiSignInfo{}
			err = proto.Unmarshal(multiSignInfoBytes, multiSignInfo)
			if err != nil {
				h.log.Errorf("unmarshal MultiSignInfo failed, multiSignReqId = %v, err = %v", multiSignReqId, err)
				return true
			}

			var calleeContractName string
			var calleeMethod string
			for _, kvpair := range multiSignInfo.Payload.Parameters {
				if kvpair.Key == syscontract.MultiReq_SYS_CONTRACT_NAME.String() {
					calleeContractName = string(kvpair.Value)
				}
				if kvpair.Key == syscontract.MultiReq_SYS_METHOD.String() {
					calleeMethod = string(kvpair.Value)
				}
			}
			if calleeContractName == syscontract.SystemContract_CONTRACT_MANAGE.String() {
				if calleeMethod == syscontract.ContractManageFunction_INIT_CONTRACT.String() ||
					calleeMethod == syscontract.ContractManageFunction_UPGRADE_CONTRACT.String() {
					h.log.Debugf("need charging gas, multiSignReqId = %v", multiSignReqId)
					return true
				}
			}
		}
	}
	h.log.Debugf("need not charging gas")
	return false
}

// GetContractFromCache retrieves contract from cache or fetches it using singleflight
func (h *CommonVMHelper) GetContractFromCache(txSimContext protocol.TxSimContext,
	contractName string, contractCache interface{}) (*commonPb.Contract, error) {
	cache := contractCache.(*sync.Map)

	var contract *commonPb.Contract
	var err error

	// Check if contract exists in cache
	if ct, ok := cache.Load(contractName); ok {
		if contract, ok = ct.(*commonPb.Contract); !ok {
			err = errors.New("failed to transfer contract from interface to struct")
			h.log.Error(err)
			return nil, err
		}
		return contract, nil
	}

	// Contract not in cache, use singleflight to get it
	ct, err, _ := sf.Do(h.chainConf.ChainConfig().ChainId+":"+contractName, func() (interface{}, error) {
		var ctTmp *commonPb.Contract
		ctTmp, err = txSimContext.GetContractByName(contractName)
		if err != nil {
			h.log.Errorf("Get contract info by name[%s] error:%s", contractName, err)
			return nil, err
		}
		// Store to contract cache after fetching
		cache.Store(contractName, ctTmp)
		return ctTmp, nil
	})

	if err != nil {
		return nil, err
	}

	if contract, ok := ct.(*commonPb.Contract); ok {
		return contract, nil
	}

	err = errors.New("failed to transfer contract from interface to struct")
	h.log.Error(err)
	return nil, err
}

// GetContractBytecode retrieves bytecode for non-native contracts
func (h *CommonVMHelper) GetContractBytecode(txSimContext protocol.TxSimContext,
	contract *commonPb.Contract) ([]byte, error) {

	if contract.RuntimeType != commonPb.RuntimeType_NATIVE &&
		contract.RuntimeType != commonPb.RuntimeType_DOCKER_GO &&
		contract.RuntimeType != commonPb.RuntimeType_GO {
		byteCode, err := txSimContext.GetContractBytecode(contract.Name)
		if err != nil {
			h.log.Errorf("Get contract bytecode by name[%s] error:%s", contract.Name, err)
			return nil, err
		}
		return byteCode, nil
	}

	h.log.DebugDynamic(func() string {
		contractData, _ := json.Marshal(contract)
		return fmt.Sprintf("contract[%s] is a native contract, definition:%s",
			contract.Name, string(contractData))
	})
	return nil, nil
}

// ParseParameter2220 parses transaction parameters (version 2220)
func (h *CommonVMHelper) ParseParameter2220(parameterPairs []*commonPb.KeyValuePair,
	checkParamsNum bool) (map[string][]byte, error) {
	// Verify parameter count
	if checkParamsNum && len(parameterPairs) > protocol.ParametersKeyMaxCount {
		return nil, fmt.Errorf("expect parameters length less than %d, but got %d",
			protocol.ParametersKeyMaxCount, len(parameterPairs))
	}

	parameters := make(map[string][]byte, 16)
	for i := 0; i < len(parameterPairs); i++ {
		key := parameterPairs[i].Key
		value := parameterPairs[i].Value

		if len(key) > protocol.DefaultMaxStateKeyLen {
			return nil, fmt.Errorf("expect key length less than %d, but got %d",
				protocol.DefaultMaxStateKeyLen, len(key))
		}

		match := h.keyReg.MatchString(key)
		if !match {
			return nil, fmt.Errorf(
				"expect key no special characters, but got key:[%s]. letter, number, dot and underline are allowed",
				key)
		}

		if len(value) > int(protocol.ParametersValueMaxLength) {
			return nil, fmt.Errorf("expect value length less than %d, but got %d",
				protocol.ParametersValueMaxLength, len(value))
		}

		parameters[key] = value
	}
	return parameters, nil
}

// ParseParameter2210 parses transaction parameters (version 2210)
func (h *CommonVMHelper) ParseParameter2210(parameterPairs []*commonPb.KeyValuePair) (map[string][]byte, error) {
	return h.ParseParameter2220(parameterPairs, true)
}

// ChargeGasLimit charges gas limit from user account
func (h *CommonVMHelper) ChargeGasLimit(accountMangerContract *commonPb.Contract, tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, contractName, method string, pk []byte,
	result *commonPb.Result, snapshot protocol.Snapshot) (*commonPb.Result, error) {

	if !h.CheckGasEnable() || !h.CheckNativeFilter(contractName, method, tx, snapshot) ||
		tx.Payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		h.log.Debugf("%s:%s no need to charge gas.", contractName, method)
		return result, nil
	}

	if tx.Payload.Limit == nil {
		err := errors.New("tx payload limit is nil")
		h.log.Error(err.Error())
		result.Message = err.Error()
		return result, err
	}

	limit := tx.Payload.Limit.GasLimit
	chargeParameters := map[string][]byte{
		accountmgr.ChargePublicKey: pk,
		accountmgr.ChargeGasAmount: []byte(strconv.FormatUint(limit, 10)),
	}

	h.log.Debugf("【chargeGasLimit】%v, pk = %s, amount = %v", tx.Payload.TxId, pk, limit)
	runChargeGasContract, _, code := h.vmManager.RunContract(
		accountMangerContract, syscontract.GasAccountFunction_CHARGE_GAS.String(),
		nil, chargeParameters, txSimContext, 0, commonPb.TxType_INVOKE_CONTRACT)

	if code != commonPb.TxStatusCode_SUCCESS {
		result.Code = code
		result.ContractResult = runChargeGasContract
		return result, errors.New(runChargeGasContract.Message)
	}

	return result, nil
}

// RefundGas refunds unused gas to user account
func (h *CommonVMHelper) RefundGas(accountMangerContract *commonPb.Contract, tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, contractName, method string, pk []byte,
	result *commonPb.Result, contractResultPayload *commonPb.ContractResult,
	snapshot protocol.Snapshot) (*commonPb.Result, error) {

	if !h.CheckGasEnable() || !h.CheckNativeFilter(contractName, method, tx, snapshot) ||
		tx.Payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return result, nil
	}

	if tx.Payload.Limit == nil {
		err := errors.New("tx payload limit is nil")
		h.log.Error(err.Error())
		result.Message = err.Error()
		return result, err
	}

	limit := tx.Payload.Limit.GasLimit
	if limit < contractResultPayload.GasUsed {
		err := fmt.Errorf("gas limit is not enough, [limit:%d]/[gasUsed:%d]",
			limit, contractResultPayload.GasUsed)
		h.log.Error(err.Error())
		result.Message = err.Error()
		return result, err
	}

	refundGas := limit - contractResultPayload.GasUsed
	h.log.Debugf("refund gas [%d], gas used [%d]", refundGas, contractResultPayload.GasUsed)

	if refundGas == 0 {
		return result, nil
	}

	refundGasParameters := map[string][]byte{
		accountmgr.RechargeKey:       pk,
		accountmgr.RechargeAmountKey: []byte(strconv.FormatUint(refundGas, 10)),
	}

	refundGasContract, _, code := h.vmManager.RunContract(
		accountMangerContract, syscontract.GasAccountFunction_REFUND_GAS_VM.String(),
		nil, refundGasParameters, txSimContext, 0, commonPb.TxType_INVOKE_CONTRACT)

	if code != commonPb.TxStatusCode_SUCCESS {
		result.Code = code
		result.ContractResult = refundGasContract
		return result, errors.New(refundGasContract.Message)
	}

	return result, nil
}

// CheckRefundGas checks gas limit and refunds if needed (for version 2300)
func (h *CommonVMHelper) CheckRefundGas(accountMangerContract *commonPb.Contract, tx *commonPb.Transaction,
	txSimContext protocol.TxSimContext, contractName, method string, pk []byte,
	result *commonPb.Result, contractResultPayload *commonPb.ContractResult,
	enableOptimizeChargeGas bool, snapshot protocol.Snapshot) error {

	// Get tx's gas limit
	limit, err := GetTxGasLimit(tx)
	if err != nil {
		h.log.Errorf("getTxGasLimit error: %v", err)
		result.Message = err.Error()
		return err
	}

	// Compare the gas used with gas limit
	if limit < contractResultPayload.GasUsed {
		err = fmt.Errorf("gas limit is not enough, [limit:%d]/[gasUsed:%d]",
			limit, contractResultPayload.GasUsed)
		h.log.Error(err.Error())
		result.ContractResult.Code = uint32(commonPb.TxStatusCode_CONTRACT_FAIL)
		result.ContractResult.Message = err.Error()
		result.ContractResult.GasUsed = limit
		result.ContractResult.Result = nil
		result.ContractResult.ContractEvent = nil
		return err
	}

	if !enableOptimizeChargeGas {
		if _, err = h.RefundGas(accountMangerContract, tx, txSimContext, contractName, method, pk, result,
			contractResultPayload, snapshot); err != nil {
			h.log.Errorf("refund gas err is %v", err)
			if txSimContext.GetBlockVersion() >= blockVersion2300 {
				result.Code = commonPb.TxStatusCode_INTERNAL_ERROR
				result.Message = err.Error()
				result.ContractResult.Code = uint32(1)
				result.ContractResult.Message = err.Error()
				result.ContractResult.ContractEvent = nil
				return err
			}
		}
	}

	return nil
}

// GetTxGasLimit extracts gas limit from transaction
func GetTxGasLimit(tx *commonPb.Transaction) (uint64, error) {
	if tx.Payload.Limit == nil {
		return 0, errors.New("tx payload limit is nil")
	}
	return tx.Payload.Limit.GasLimit, nil
}

// getMultiSignEnableManualRun checks if manual run is enabled for multi-sign
func getMultiSignEnableManualRun(chainConfig *configPb.ChainConfig) bool {
	if chainConfig.Vm == nil {
		return false
	} else if chainConfig.Vm.Native == nil {
		return false
	} else if chainConfig.Vm.Native.Multisign == nil {
		return false
	}
	return chainConfig.Vm.Native.Multisign.EnableManualRun
}

// GetAccountMgrContractAndPk gets account manager contract and public key
func (h *CommonVMHelper) GetAccountMgrContractAndPk(txSimContext protocol.TxSimContext,
	tx *commonPb.Transaction, contractName, method string,
	snapshot protocol.Snapshot, ac protocol.AccessControlProvider) (*commonPb.Contract, []byte, error) {

	if !h.CheckGasEnable() || !h.CheckNativeFilter(contractName, method, tx, snapshot) ||
		tx.Payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return nil, nil, nil
	}

	h.log.Debugf("getAccountMgrContractAndPk => txSimContext.GetContractByName(`%s`)",
		syscontract.SystemContract_ACCOUNT_MANAGER.String())
	accountMangerContract, err := txSimContext.GetContractByName(syscontract.SystemContract_ACCOUNT_MANAGER.String())
	if err != nil {
		h.log.Error(err.Error())
		return nil, nil, err
	}

	_, publicKey, err := schedulerUtils.GetPayerAddressAndPkFromTx(tx, snapshot, ac)
	if err != nil {
		h.log.Error(err.Error())
		return accountMangerContract, nil, err
	}
	pk, err := publicKey.String()
	if err != nil {
		h.log.Error(err.Error())
		return accountMangerContract, nil, err
	}
	return accountMangerContract, []byte(pk), nil
}

// ReleaseContractCache releases all contracts from cache
func (h *CommonVMHelper) ReleaseContractCache() {
	h.contractCache.Range(func(key interface{}, value interface{}) bool {
		h.contractCache.Delete(key)
		return true
	})
}

// GuardForExecuteTx2220 filters out txs that need not go into runVM
func (h *CommonVMHelper) GuardForExecuteTx2220(tx *commonPb.Transaction, txSimContext protocol.TxSimContext,
	enableGas bool, enableOptimizedChargeGas bool) bool {
	if tx.Result != nil && tx.Result.Code == commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED {
		if enableOptimizedChargeGas {
			txSimContext.SetTxResult(tx.Result)
			return false
		}
	}
	return true
}

// GuardForExecuteTx2300 filters out txs that need not go into runVM (version 2300)
func (h *CommonVMHelper) GuardForExecuteTx2300(tx *commonPb.Transaction, txSimContext protocol.TxSimContext,
	enableGas bool, enableOptimizeChargeGas bool, snapshot protocol.Snapshot) bool {

	txNeedChargeGas := h.CheckNativeFilter(
		tx.Payload.ContractName,
		tx.Payload.Method,
		tx,
		snapshot)

	if enableOptimizeChargeGas {
		// below code is in charge_gas_optimize mode

		// need charge gas, but gasLimit is not set
		if txNeedChargeGas && tx.Payload.Limit == nil {
			if tx.Result == nil {
				// `proposer node` has filter txs out in `dispatchTxsInSenderCollection` function
				h.log.Errorf("proposer node with `enable_optimize_gas` mode can't get tx that's gas_limit is nil in this place.")
			} else {
				// `verify node` should return error result same with `proposer node` do in `dispatchTxsInSenderCollection`
				txResult := &commonPb.Result{
					Code: commonPb.TxStatusCode_GAS_LIMIT_NOT_SET,
					ContractResult: &commonPb.ContractResult{
						Code:    uint32(1),
						Result:  nil,
						Message: ErrMsgOfGasLimitNotSet,
						GasUsed: uint64(0),
					},
					RwSetHash: nil,
					Message:   ErrMsgOfGasLimitNotSet,
				}
				txSimContext.SetTxResult(txResult)
				return false
			}
		} else if txNeedChargeGas && tx.Payload.Limit != nil {
			// in `proposer node`:
			// 	1) tx.Result should be set by `dispatchTxsInSenderCollection()`
			//  2) tx.Result should be set by `runVM()`
			// in `verify node`:
			//  1) tx.Result should be set in this place
			//  2) tx.Result should be set in `runVM()` later
			if tx.Result != nil && tx.Result.Code == commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED {
				pk, _ := schedulerUtils.GetPkFromTx(tx, snapshot)
				chainCfg := h.chainConf.ChainConfig()
				addr, _ := schedulerUtils.PublicKeyToAddress(pk, chainCfg)
				h.log.Debugf("balance is too low to execute tx. address = %v, public key = %s", addr, pk)
				errMsg := fmt.Sprintf("`%s` has no enough balance to execute tx.", addr)
				txResult := &commonPb.Result{
					Code: commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED,
					ContractResult: &commonPb.ContractResult{
						Code:    uint32(1),
						Result:  nil,
						Message: errMsg,
						GasUsed: uint64(0),
					},
					RwSetHash: nil,
					Message:   errMsg,
				}
				txSimContext.SetTxResult(txResult)
				return false
			}
		}
	} else if enableGas {
		// below code is in charge_gas mode

		if txNeedChargeGas && tx.Payload.Limit == nil {
			txResult := &commonPb.Result{
				Code: commonPb.TxStatusCode_GAS_LIMIT_NOT_SET,
				ContractResult: &commonPb.ContractResult{
					Code:    uint32(1),
					Result:  nil,
					Message: ErrMsgOfGasLimitNotSet,
					GasUsed: uint64(0),
				},
				RwSetHash: nil,
				Message:   ErrMsgOfGasLimitNotSet,
			}
			// `proposer node` need set result into tx and txSimContext
			// `verify node` need set result into txSimContext
			txSimContext.SetTxResult(txResult)
			if tx.Result == nil {
				tx.Result = txResult
			}
			return false
		}
	}

	return true
}

// RunVMContext contains context needed for running VM
type RunVMContext struct {
	Tx                      *commonPb.Transaction
	TxSimContext            protocol.TxSimContext
	EnableOptimizeChargeGas bool
	Snapshot                protocol.Snapshot
	AC                      protocol.AccessControlProvider // Can be nil for SerialScheduler
	ContractCache           interface{}
}

// RunVM2300 executes transaction in VM (version 2300)
func (h *CommonVMHelper) RunVM2300(ctx *RunVMContext) (*commonPb.Result, protocol.ExecOrderTxType, error) {
	var (
		contractName          string
		method                string
		byteCode              []byte
		pk                    []byte
		specialTxType         protocol.ExecOrderTxType
		accountMangerContract *commonPb.Contract
		contractResultPayload *commonPb.ContractResult
		txStatusCode          commonPb.TxStatusCode
		contract              *commonPb.Contract
	)

	h.log.Debugf("runVM =>  for tx `%v`", ctx.Tx.GetPayload().TxId)
	result := &commonPb.Result{
		Code: commonPb.TxStatusCode_SUCCESS,
		ContractResult: &commonPb.ContractResult{
			Code:    uint32(0),
			Result:  nil,
			Message: "",
		},
		RwSetHash: nil,
	}
	payload := ctx.Tx.Payload
	if payload.TxType != commonPb.TxType_QUERY_CONTRACT && payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return errResult(result, fmt.Errorf("no such tx type: %s", ctx.Tx.Payload.TxType))
	}

	contractName = payload.ContractName
	method = payload.Method
	parameters, err := h.ParseParameter2220(payload.Parameters, !ctx.EnableOptimizeChargeGas)
	if err != nil {
		h.log.Errorf("parse contract[%s] parameters error:%s", contractName, err)
		return errResult(result, fmt.Errorf(
			"parse tx[%s] contract[%s] parameters error:%s",
			payload.TxId,
			contractName,
			err.Error()),
		)
	}

	h.log.Debugf("runVM => txSimContext.GetContractByName(`%s`) for tx `%v`", contractName, ctx.Tx.GetPayload().TxId)

	if contract, err = h.GetContractFromCache(ctx.TxSimContext, contractName, ctx.ContractCache); err != nil {
		return errResult(result, err)
	}

	if byteCode, err = h.GetContractBytecode(ctx.TxSimContext, contract); err != nil {
		return errResult(result, err)
	}

	if h.CheckGasEnable() && !ctx.EnableOptimizeChargeGas {
		accountMangerContract, pk, err = h.GetAccountMgrContractAndPk(ctx.TxSimContext, ctx.Tx, contract.Name, method,
			ctx.Snapshot, ctx.AC)
		if err != nil {
			return result, specialTxType, err
		}

		_, err = h.ChargeGasLimit(accountMangerContract, ctx.Tx, ctx.TxSimContext, contract.Name, method, pk, result, ctx.Snapshot)
		if err != nil {
			h.log.Errorf("charge gas limit err is %v", err)
			result.Code = commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED
			result.Message = err.Error()
			result.ContractResult.Code = uint32(1)
			result.ContractResult.Message = err.Error()
			return result, specialTxType, err
		}
	}

	contractResultPayload, specialTxType, txStatusCode = h.vmManager.RunContract(contract, method, byteCode,
		parameters, ctx.TxSimContext, 0, ctx.Tx.Payload.TxType)
	result.Code = txStatusCode
	result.ContractResult = contractResultPayload

	// refund gas
	if h.CheckGasEnable() {
		// check if this invoke needs charging gas
		if !h.CheckNativeFilter(contract.Name, method, ctx.Tx, ctx.Snapshot) {
			return result, specialTxType, err
		}

		// check and refund gas
		if err = h.CheckRefundGas(accountMangerContract, ctx.Tx, ctx.TxSimContext, contractName, method, pk, result,
			contractResultPayload, ctx.EnableOptimizeChargeGas, ctx.Snapshot); err != nil {
			return result, specialTxType, err
		}
	}

	if txStatusCode == commonPb.TxStatusCode_SUCCESS {
		return result, specialTxType, nil
	}
	return result, specialTxType, errors.New(contractResultPayload.Message)
}

// RunVM2220 executes transaction in VM (version 2220)
func (h *CommonVMHelper) RunVM2220(ctx *RunVMContext) (*commonPb.Result, protocol.ExecOrderTxType, error) {
	var (
		contractName          string
		method                string
		byteCode              []byte
		pk                    []byte
		specialTxType         protocol.ExecOrderTxType
		accountMangerContract *commonPb.Contract
		contractResultPayload *commonPb.ContractResult
		txStatusCode          commonPb.TxStatusCode
	)

	h.log.Debugf("runVM =>  for tx `%v`", ctx.Tx.GetPayload().TxId)
	result := &commonPb.Result{
		Code: commonPb.TxStatusCode_SUCCESS,
		ContractResult: &commonPb.ContractResult{
			Code:    uint32(0),
			Result:  nil,
			Message: "",
		},
		RwSetHash: nil,
	}
	payload := ctx.Tx.Payload
	if payload.TxType != commonPb.TxType_QUERY_CONTRACT && payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return errResult(result, fmt.Errorf("no such tx type: %s", ctx.Tx.Payload.TxType))
	}

	contractName = payload.ContractName
	method = payload.Method
	parameters, err := h.ParseParameter2220(payload.Parameters, !ctx.EnableOptimizeChargeGas)
	if err != nil {
		h.log.Errorf("parse contract[%s] parameters error:%s", contractName, err)
		return errResult(result, fmt.Errorf(
			"parse tx[%s] contract[%s] parameters error:%s",
			payload.TxId,
			contractName,
			err.Error()),
		)
	}

	h.log.Debugf("runVM => txSimContext.GetContractByName(`%s`) for tx `%v`", contractName, ctx.Tx.GetPayload().TxId)
	contract, err := h.GetContractFromCache(ctx.TxSimContext, contractName, ctx.ContractCache)
	if err != nil {
		h.log.Errorf("Get contract info by name[%s] error:%s", contractName, err)
		return errResult(result, err)
	}

	byteCode, err = h.GetContractBytecode(ctx.TxSimContext, contract)
	if err != nil {
		return errResult(result, err)
	}

	if h.CheckGasEnable() && !ctx.EnableOptimizeChargeGas {
		accountMangerContract, pk, err = h.GetAccountMgrContractAndPk(ctx.TxSimContext, ctx.Tx, contract.Name, method,
			ctx.Snapshot, ctx.AC)
		if err != nil {
			return result, specialTxType, err
		}

		// charge gas limit
		_, err = h.ChargeGasLimit(accountMangerContract, ctx.Tx, ctx.TxSimContext, contract.Name, method, pk, result, ctx.Snapshot)
		if err != nil {
			h.log.Errorf("charge gas limit err is %v", err)
			result.Code = commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED
			result.Message = err.Error()
			result.ContractResult.Code = uint32(1)
			result.ContractResult.Message = err.Error()
			return result, specialTxType, err
		}
	}

	contractResultPayload, specialTxType, txStatusCode = h.vmManager.RunContract(contract, method, byteCode,
		parameters, ctx.TxSimContext, 0, ctx.Tx.Payload.TxType)
	result.Code = txStatusCode
	result.ContractResult = contractResultPayload

	// refund gas
	if h.CheckGasEnable() {
		// check if this invoke needs charging gas
		if !h.CheckNativeFilter(contract.Name, method, ctx.Tx, ctx.Snapshot) {
			return result, specialTxType, err
		}

		// get tx's gas limit
		limit, err := GetTxGasLimit(ctx.Tx)
		if err != nil {
			h.log.Errorf("getTxGasLimit error: %v", err)
			result.Message = err.Error()
			return result, specialTxType, err
		}

		// compare the gas used with gas limit
		if limit < contractResultPayload.GasUsed {
			err = fmt.Errorf("gas limit is not enough, [limit:%d]/[gasUsed:%d]",
				limit, contractResultPayload.GasUsed)
			h.log.Error(err.Error())
			result.ContractResult.Code = uint32(commonPb.TxStatusCode_CONTRACT_FAIL)
			result.ContractResult.Message = err.Error()
			result.ContractResult.GasUsed = limit
			return result, specialTxType, err
		}
		if !ctx.EnableOptimizeChargeGas {
			if _, err = h.RefundGas(accountMangerContract, ctx.Tx, ctx.TxSimContext, contractName, method, pk, result,
				contractResultPayload, ctx.Snapshot); err != nil {
				h.log.Errorf("refund gas err is %v", err)
			}
		}
	}

	if txStatusCode == commonPb.TxStatusCode_SUCCESS {
		return result, specialTxType, nil
	}
	return result, specialTxType, errors.New(contractResultPayload.Message)
}

// RunVM2210 executes transaction in VM (version 2210)
func (h *CommonVMHelper) RunVM2210(ctx *RunVMContext) (*commonPb.Result, protocol.ExecOrderTxType, error) {
	var (
		contractName          string
		method                string
		byteCode              []byte
		pk                    []byte
		specialTxType         protocol.ExecOrderTxType
		accountMangerContract *commonPb.Contract
		contractResultPayload *commonPb.ContractResult
		txStatusCode          commonPb.TxStatusCode
	)

	result := &commonPb.Result{
		Code: commonPb.TxStatusCode_SUCCESS,
		ContractResult: &commonPb.ContractResult{
			Code:    uint32(0),
			Result:  nil,
			Message: "",
		},
		RwSetHash: nil,
	}
	payload := ctx.Tx.Payload
	if payload.TxType != commonPb.TxType_QUERY_CONTRACT && payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return errResult(result, fmt.Errorf("no such tx type: %s", ctx.Tx.Payload.TxType))
	}

	contractName = payload.ContractName
	method = payload.Method
	parameters, err := h.ParseParameter2210(payload.Parameters)
	if err != nil {
		h.log.Errorf("parse contract[%s] parameters error:%s", contractName, err)
		return errResult(result, fmt.Errorf(
			"parse tx[%s] contract[%s] parameters error:%s",
			payload.TxId,
			contractName,
			err.Error()),
		)
	}

	contract, err := h.GetContractFromCache(ctx.TxSimContext, contractName, ctx.ContractCache)
	if err != nil {
		h.log.Errorf("Get contract info by name[%s] error:%s", contractName, err)
		return errResult(result, err)
	}

	byteCode, err = h.GetContractBytecode(ctx.TxSimContext, contract)
	if err != nil {
		return errResult(result, err)
	}

	accountMangerContract, pk, err = h.GetAccountMgrContractAndPk(ctx.TxSimContext, ctx.Tx, contractName, method,
		ctx.Snapshot, ctx.AC)
	if err != nil {
		return result, specialTxType, err
	}

	// charge gas limit
	_, err = h.ChargeGasLimit(accountMangerContract, ctx.Tx, ctx.TxSimContext, contractName, method, pk, result, ctx.Snapshot)
	if err != nil {
		h.log.Errorf("charge gas limit err is %v", err)
		result.Code = commonPb.TxStatusCode_GAS_BALANCE_NOT_ENOUGH_FAILED
		result.Message = err.Error()
		result.ContractResult.Code = uint32(1)
		result.ContractResult.Message = err.Error()
		return result, specialTxType, err
	}

	contractResultPayload, specialTxType, txStatusCode = h.vmManager.RunContract(contract, method, byteCode,
		parameters, ctx.TxSimContext, 0, ctx.Tx.Payload.TxType)
	result.Code = txStatusCode
	result.ContractResult = contractResultPayload

	// refund gas
	_, err = h.RefundGas(accountMangerContract, ctx.Tx, ctx.TxSimContext, contractName, method, pk, result,
		contractResultPayload, ctx.Snapshot)
	if err != nil {
		h.log.Errorf("refund gas err is %v", err)
	}

	if txStatusCode == commonPb.TxStatusCode_SUCCESS {
		return result, specialTxType, nil
	}
	return result, specialTxType, errors.New(contractResultPayload.Message)
}

// ExecuteTx execute tx
// the main sequence is:
// 1. init sim context
// 2. gas filter
// 3. execute tx based on different version
func (h *CommonVMHelper) ExecuteTx(tx *commonPb.Transaction, snapshot protocol.Snapshot, block *commonPb.Block) (protocol.TxSimContext,
	protocol.ExecOrderTxType, bool) {

	blockVersion := block.GetHeader().BlockVersion

	// STEP1: init sim-context
	txSimContext := vm.NewTxSimContext(h.vmManager, snapshot, tx, blockVersion, h.log)
	h.log.DebugDynamic(func() string {
		return fmt.Sprintf("NewTxSimContext finished for tx id:%s,tx.Result =%v", tx.Payload.GetTxId(), tx.Result)
	})
	// STEP2: tx check, including gas
	enableGas := h.CheckGasEnable()
	enableOptimizeChargeGas := coinbasemgr.IsOptimizeChargeGasEnabled(h.chainConf)
	if blockVersion >= blockVersion2300 {
		if !h.GuardForExecuteTx2300(tx, txSimContext, enableGas, enableOptimizeChargeGas, snapshot) {
			return txSimContext, protocol.ExecOrderTxTypeNormal, false
		}
	} else if blockVersion >= 2220 {
		if !h.GuardForExecuteTx2220(tx, txSimContext, enableGas, enableOptimizeChargeGas) {
			return txSimContext, protocol.ExecOrderTxTypeNormal, false
		}
	}

	runVmSuccess := true
	var txResult *commonPb.Result
	var err error
	var specialTxType protocol.ExecOrderTxType

	// STEP3: run tx based on different block version
	h.log.Debugf("run vm start for tx:%s", tx.Payload.GetTxId())

	// Create context for VM execution
	ctx := &RunVMContext{
		Tx:                      tx,
		TxSimContext:            txSimContext,
		EnableOptimizeChargeGas: enableOptimizeChargeGas,
		Snapshot:                snapshot,
		AC:                      h.ac,
		ContractCache:           h.contractCache,
	}

	if blockVersion >= 2300 {
		if txResult, specialTxType, err = h.RunVM2300(ctx); err != nil {
			runVmSuccess = false
			h.log.Errorf("failed to run vm for tx id:%s,contractName:%s, tx result:%+v, error:%+v",
				tx.Payload.GetTxId(), tx.Payload.ContractName, txResult, err)
		}
	} else if blockVersion >= 2220 {
		if txResult, specialTxType, err = h.RunVM2220(ctx); err != nil {
			runVmSuccess = false
			h.log.Errorf("failed to run vm for tx id:%s,contractName:%s, tx result:%+v, error:%+v",
				tx.Payload.GetTxId(), tx.Payload.ContractName, txResult, err)
		}
	} else {
		if txResult, specialTxType, err = h.RunVM2210(ctx); err != nil {
			runVmSuccess = false
			h.log.Errorf("failed to run vm for tx id:%s,contractName:%s, tx result:%+v, error:%+v",
				tx.Payload.GetTxId(), tx.Payload.ContractName, txResult, err)
		}
	}
	h.log.Debugf("run vm finished for tx:%s, runVmSuccess:%v, txResult = %v ", tx.Payload.TxId, runVmSuccess, txResult)
	txSimContext.SetTxResult(txResult)
	return txSimContext, specialTxType, runVmSuccess

}

// todo：先假设read ERROR 的逻辑能走的通，后续再测试验证是否真的能走通

// ExecuteTxForBlockSTM is like ExecuteTx, but it returns the underlying VM error instead of swallowing it.
// This is used by Block-STM to distinguish dependency (READ_ERROR) from genuine execution failures.
func (h *CommonVMHelper) ExecuteTxForBlockSTM(tx *commonPb.Transaction, snapshot protocol.Snapshot, block *commonPb.Block) (protocol.TxSimContext,
	protocol.ExecOrderTxType, bool, error) { // 注意这里多了个error

	blockVersion := block.GetHeader().BlockVersion

	// STEP1: init sim-context
	txSimContext := vm.NewTxSimContext(h.vmManager, snapshot, tx, blockVersion, h.log)

	// STEP2: tx check, including gas
	enableGas := h.CheckGasEnable()
	enableOptimizeChargeGas := coinbasemgr.IsOptimizeChargeGasEnabled(h.chainConf)
	if blockVersion >= blockVersion2300 {
		if !h.GuardForExecuteTx2300(tx, txSimContext, enableGas, enableOptimizeChargeGas, snapshot) {
			return txSimContext, protocol.ExecOrderTxTypeNormal, false, nil
		}
	} else if blockVersion >= 2220 {
		if !h.GuardForExecuteTx2220(tx, txSimContext, enableGas, enableOptimizeChargeGas) {
			return txSimContext, protocol.ExecOrderTxTypeNormal, false, nil
		}
	}

	// STEP3: run tx based on different block version
	ctx := &RunVMContext{
		Tx:                      tx,
		TxSimContext:            txSimContext,
		EnableOptimizeChargeGas: enableOptimizeChargeGas,
		Snapshot:                snapshot,
		AC:                      h.ac,
		ContractCache:           h.contractCache,
	}

	runVmSuccess := true
	var txResult *commonPb.Result
	var err error
	var specialTxType protocol.ExecOrderTxType

	if blockVersion >= 2300 {
		txResult, specialTxType, err = h.RunVM2300(ctx)
	} else if blockVersion >= 2220 {
		txResult, specialTxType, err = h.RunVM2220(ctx)
	} else {
		txResult, specialTxType, err = h.RunVM2210(ctx)
	}

	if err != nil {
		runVmSuccess = false
	}

	// refactor开始
	// Do not set tx result on dependency/read-block error; the scheduler will re-execute later.
	var blocked interface{ BlockingTxnIndex() int }
	if errors.As(err, &blocked) {
		return txSimContext, specialTxType, runVmSuccess, err
	}
	if blockingIdx, ok := parseBlockSTMReadBlocked(err); ok {
		return txSimContext, specialTxType, runVmSuccess, &blockSTMReadBlockedError{blockingTxnIdx: blockingIdx}
	}
	// refactor结束

	// Set tx result for non-blocking outcomes (including ordinary execution failures).
	txSimContext.SetTxResult(txResult)
	return txSimContext, specialTxType, runVmSuccess, err
}
