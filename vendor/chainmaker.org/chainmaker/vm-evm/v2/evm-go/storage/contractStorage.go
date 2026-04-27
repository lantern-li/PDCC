/*
 * Copyright 2020 The SealEVM Authors
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package storage

import (
	"encoding/hex"
	"fmt"

	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/vm-evm/v2/evm-go/params"

	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"

	"chainmaker.org/chainmaker/common/v2/evmutils"
	"chainmaker.org/chainmaker/vm-evm/v2/evm-go/environment"
)

type ContractStorage struct {
	OutParams *CrossVmParams
	//InParams        *CrossVmParams
	ResultCache ResultCache
	//ExternalStorage IExternalStorage
	readOnlyCache readOnlyCache
	Ctx           protocol.TxSimContext
	BlockHash     *evmutils.Int
	Contract      *common.Contract // contract info
	SystemLog     protocol.Logger
}

func NewStorage(extStorage IExternalStorage) *ContractStorage {
	s := &ContractStorage{
		ResultCache: ResultCache{
			//OriginalData: CacheUnderAddress{},
			WriteCache: CacheUnderAddress{},
			ReadCache:  CacheUnderAddress{},
			Balance:    BalanceCache{},
			Logs:       LogCache{},
			Destructs:  Cache{},
		},
		//ExternalStorage: extStorage,
		readOnlyCache: readOnlyCache{
			Code:      CodeCache{},
			CodeSize:  Cache{},
			CodeHash:  Cache{},
			BlockHash: Cache{},
		},
	}
	return s
}

//func (c *ContractStorage) GetBalance(address *evmutils.Int) (*evmutils.Int, error) {
//	return evmutils.New(0), nil
//}

func (c *ContractStorage) GetBalance(address *evmutils.Int) (*evmutils.Int, error) {
	v := c.GetCurrentBlockVersion()
	if v <= params.V2217 || v == params.V2300 || v == params.V2030100 {
		//2300 and 2310 have been released, but 2217 has found bugs, so versions before 2218, as well as 2300 and 2310
		//that have been released, use the old logic, and other versions use the new logic
		return evmutils.New(0), nil
	}

	var manager common.Contract
	method := syscontract.GasAccountFunction_GET_BALANCE.String()
	manager.Name = syscontract.SystemContract_ACCOUNT_MANAGER.String()
	manager.Status = common.ContractStatus_NORMAL

	parameters := make(map[string][]byte)
	//parameters["address_key"] = []byte(hex.EncodeToString(address.Bytes()))
	parameters["address_key"] = []byte(IntAddr2HexStr(address, v))

	b := evmutils.New(0)
	res, _, stat := c.Ctx.CallContract(c.Contract, &manager, method, nil, parameters, 0,
		common.TxType_INVOKE_CONTRACT)
	if stat != common.TxStatusCode_SUCCESS {
		return b, fmt.Errorf("failed to get balance of %s", hex.EncodeToString(address.Bytes()))
	}

	b.SetBytes(res.Result)
	return b, nil
}

func (c *ContractStorage) CanTransfer(from, to, val *evmutils.Int) bool {
	return false
}

func (c *ContractStorage) GetCode(address *evmutils.Int) (code []byte, err error) {
	//return utils.GetContractBytecode(c.Ctx.Get, address.String())
	key := IntAddr2HexStr(address, c.GetCurrentBlockVersion())
	if c.GetCurrentBlockVersion() >= params.V2220 {
		////version >= v2.2.0 code stored by name, so get contract first
		//contract, err := c.Ctx.GetContractByName(hex.EncodeToString(address.Bytes()))
		contract, err := c.Ctx.GetContractByName(key)
		if err != nil {
			return nil, err
		}
		key = contract.Name
	}

	return c.Ctx.GetContractBytecode(key)
}

func (c *ContractStorage) GetCodeSize(address *evmutils.Int) (size *evmutils.Int, err error) {
	code, err := c.GetCode(address)
	if err != nil {
		c.SystemLog.Error("failed to get other contract code size :", err.Error())
		return nil, err
	}
	return evmutils.New(int64(len(code))), err
}

func (c *ContractStorage) GetCodeHash(address *evmutils.Int) (codeHase *evmutils.Int, err error) {
	code, err := c.GetCode(address)
	if err != nil {
		c.SystemLog.Error("failed to get other contract code hash :", err.Error())
		return nil, err
	}
	hash := evmutils.Keccak256(code)
	i := evmutils.New(0)
	i.SetBytes(hash)
	return i, err
	//return evmutils.New(int64(len(code))), err
}

func (c *ContractStorage) GetBlockHash(block *evmutils.Int) (*evmutils.Int, error) {
	currentHight := c.Ctx.GetBlockHeight() - 1
	high := evmutils.MinI(int64(currentHight), block.Int64())
	Block, err := c.Ctx.GetBlockchainStore().GetBlock(uint64(high))
	if err != nil {
		return evmutils.New(0), err
	}
	hash, err := evmutils.HashBytesToEVMInt(Block.GetHeader().GetBlockHash())
	if err != nil {
		return evmutils.New(0), err
	}
	return hash, nil
}

func (c *ContractStorage) GetCurrentBlockVersion() uint32 {
	return c.Ctx.GetBlockVersion()
}

////Create a unified address generation method within EVM to avoid duplicate wheels
//func generateAddress(data []byte, addrType int32) *evmutils.Int {
//	if addrType == int32(config.AddrType_ZXL) {
//		addr, _ := evmutils.ZXAddress(data)
//		return evmutils.FromHexString(addr[2:])
//	} else {
//		return evmutils.MakeAddress(data)
//	}
//}

//This is mostly called after 2300
//func (c *ContractStorage) CreateAddress(name *evmutils.Int, addrType int32) *evmutils.Int {
//	//in seal abc smart assets application, we always create fixed contract address.
//	data := name.Bytes()
//	//return generateAddress(data, addrType)
//	addr, _ := utils.GenerateAddrInt(data, config.AddrType(addrType))
//	return addr
//}

// Only versions < 2300 are called
func (c *ContractStorage) CreateFixedAddress(caller *evmutils.Int, salt *evmutils.Int, tx environment.Transaction, addrType int32) *evmutils.Int {
	data := append(caller.Bytes(), tx.TxHash...)
	if salt != nil {
		data = append(data, salt.Bytes()...)
	}

	//return generateAddress(data, addrType)
	addr, _ := utils.NameToAddrInt(string(data), config.AddrType(addrType), 2299)
	return addr
}

func (c *ContractStorage) Load(n string, k string) (*evmutils.Int, error) {
	var val []byte
	var err error
	if c.Ctx.GetBlockVersion() < 2300 {
		//version < 2300, cross call occurs inside the vm, so there will be multiple contrats ant it's address
		val, err = c.Ctx.Get(n, []byte(k))
	} else {
		//version >= 2300, cross call will be through the chain, so each vm has only one contract name
		val, err = c.Ctx.Get(c.Contract.Name, []byte(k))
	}

	if err != nil {
		return nil, err
	}

	r := evmutils.New(0)
	r.SetBytes(val)

	return r, err
}

func (c ContractStorage) Store(address string, key string, val []byte) {
	if c.Ctx.GetBlockVersion() < 2300 {
		//version < 2300, cross call occurs inside the vm, so there will be multiple contrats ant it's address
		_ = c.Ctx.Put(address, []byte(key), val)
	} else {
		//version >= 2300, cross call will be through the chain, so each vm has only one contract name
		err := c.Ctx.Put(c.Contract.Name, []byte(key), val)
		if err != nil && c.Ctx.GetBlockVersion() >= params.V2030500 {
			panic(fmt.Sprintf("store failed, contract[%s], key[%s], value[%s]", c.Contract.Name, key, string(val)))
		}
	}
}

func (c ContractStorage) IsCrossVmMode() bool {
	//Query whether the parameter transfer mode of cross-vm contract invocation is enabled, which is used by
	//sload directives and sstore directives to distinguish regular state read/write from read/write parameters
	return c.OutParams.IsCrossVm
}

//func (c ContractStorage) GetCrossVmInParam(key string) []byte {
//	return c.InParams.ParamsCache[key]
//}

/*
	SetCrossVmOutParams

State Enumeration:

		0: None, 1: Key, 2: Value
		1. After the method completes processing, set the state enumeration to Key.
		2. Distinguish whether an elementis part of a Keyor Value.
		3. Add a parameter in outputParamto determine if the state is Key:
		   3.1 If indexis a number, the Keyis complete.
		   3.2 If indexis not a number, the Keyconsists of multiple parts:
		   → Concatenate strings until the next indexis a number and elementequals string length * 2 + 1.
		   3.3 After Keyprocessing completes, set the state enumeration to Value.
		4. If the state is Value, process as follows:
		   4.1 If indexis a number, the Valueis complete.
		   4.2 If indexis not a number, the Valueconsists of multiple parts:
		   → Concatenate strings until the next indexis a number and elementequals string length * 2 + 1.
	       4.3 After Valueprocessing completes, reset the state enumeration to Key.
*/
func (c *ContractStorage) SetCrossVmOutParams(index *evmutils.Int, element *evmutils.Int) {
	var val []byte

	if !element.IsInt64() && !element.IsUint64() {
		//If the element is not a number, the element value is obtained after whitespace is removed
		val = TruncateNullTail(element.Bytes())
	}
	if string(val) == CrossVmOutParamsBeginKey {
		//Turn on the pass parameter flag if the element is invoked by an external cross-vm contract
		c.OutParams.IsCrossVm = true
		//Marks the slot at which the parameter is started
		c.OutParams.ParamsBegin = index.Int64()
		//OutParams starts writing
		c.OutParams.SetParam(CrossVmOutParamsBeginKey, []byte("start"))
		return
	}
	if (index.Int64()-c.OutParams.ParamsBegin == 1) && !element.IsInt64() {
		//The 0th element is the cross-vm contract invocation token, and the first element is the invoked method
		c.OutParams.SetParam(CrossVmCallMethodKey, val)
		c.OutParams.State = StateKey // 设置下一个状态为Key
		return
	}
	if c.OutParams.State == StateKey && len(c.OutParams.LastParamKey) > 0 && index.IsInt64() && element.IsInt64() {
		if int64(len(c.OutParams.LastParamKey)*2+1) != element.Int64() {
			panic(fmt.Sprintf("Invalid length key at index %v", index.Bytes()))
		}
		c.OutParams.State = StateValue
		return
	}
	if c.OutParams.State == StateValue && len(c.OutParams.LastParamKey) > 0 && index.IsInt64() && element.IsInt64() {
		val := c.OutParams.GetParam(c.OutParams.LastParamKey)
		if len(val) > 0 {
			if int64(len(val)*2+1) != element.Int64() {
				panic(fmt.Sprintf("Invalid length key at index %v", index.Bytes()))
			}
			c.OutParams.State = StateKey
			c.OutParams.LastParamKey = ""
			c.OutParams.LongStrLen = 0
			return
		}
	}
	// 状态机处理
	switch c.OutParams.State {
	case StateKey:
		c.processKeyState(index, element, val)
	case StateValue:
		c.processValueState(index, element, val)
	}

}

// 处理Key状态
func (c *ContractStorage) processKeyState(index, element *evmutils.Int, val []byte) {
	if val == nil {
		val = TruncateNullTail(element.Bytes())
	}
	if index.IsInt64() {
		// 完整的Key
		c.OutParams.LastParamKey = string(val)
		c.OutParams.State = StateValue // 下一个状态为Length
	} else {
		// Key由多部分组成
		c.OutParams.LastParamKey += string(val)
		// 保持Key状态直到遇到数字index
	}
}

// 处理Value状态
func (c *ContractStorage) processValueState(index, element *evmutils.Int, val []byte) {
	if val == nil {
		val = TruncateNullTail(element.Bytes())
	}
	if index.IsInt64() {
		// 完整的Value
		c.OutParams.SetParam(c.OutParams.LastParamKey, val)
		c.OutParams.State = StateKey // 重置为Key状态
		c.OutParams.LastParamKey = ""
		c.OutParams.LongStrLen = 0
	} else {
		currentVal := c.OutParams.GetParam(c.OutParams.LastParamKey)
		if currentVal == nil {
			currentVal = val
		} else {
			currentVal = append(currentVal, val...)
		}
		c.OutParams.SetParam(c.OutParams.LastParamKey, currentVal)

	}
}

func (c ContractStorage) CallContract(name string, rtType int32, method string, byteCode []byte,
	parameters map[string][]byte, gasUsed uint64, isCreate bool) (res *common.ContractResult, stat common.TxStatusCode) {

	//Parameter storage mode is enabled only when the contract is invoked across virtual machines
	if c.OutParams.IsCrossVm {
		for k, v := range c.OutParams.ParamsCache {
			parameters[k] = v
			c.SystemLog.Debugf("cross vm call ParamsCache k: %s, v: %x", k, v)
		}
	}
	caller := &common.Contract{
		Address: string(parameters[syscontract.CrossParams_SENDER.String()]),
	}

	if isCreate {
		//If the cross-contract invocation is creating the contract, you are actually calling
		//the installContract method that manages the contract
		var contract common.Contract
		method = syscontract.ContractManageFunction_INIT_CONTRACT.String()
		contract.Name = syscontract.SystemContract_CONTRACT_MANAGE.String()
		contract.Status = common.ContractStatus_NORMAL

		parameters[syscontract.InitContract_CONTRACT_NAME.String()] = []byte(name)
		parameters[syscontract.InitContract_CONTRACT_VERSION.String()] = []byte("1.0.0")
		parameters[syscontract.InitContract_CONTRACT_RUNTIME_TYPE.String()] = []byte(common.RuntimeType(rtType).String())
		parameters[syscontract.InitContract_CONTRACT_BYTECODE.String()] = byteCode
		res, _, stat = c.Ctx.CallContract(caller, &contract, method, byteCode, parameters, gasUsed, common.TxType_INVOKE_CONTRACT)
	} else {
		c.SystemLog.Debugf("cross vm call contract name :%s ", name)
		c.SystemLog.Debugf("cross vm call contract method %s", method)
		for k, v := range parameters {
			c.SystemLog.Debugf("cross vm call contract parameter[%s]: %x", k, v)
		}
		contract, err := c.Ctx.GetContractByName(name)
		if err != nil && c.Ctx.GetBlockVersion() >= params.V2030500 {
			return &common.ContractResult{
				Code:    uint32(1),
				Result:  nil,
				Message: fmt.Sprintf("get contract by name[%s] failed", name),
				GasUsed: gasUsed,
			}, common.TxStatusCode_CONTRACT_FAIL

		}
		if c.OutParams.IsCrossVm {
			//Method is not required to create a contract. The init_contract method is automatically called
			method = string(parameters[CrossVmCallMethodKey])
		}
		// For versions above 2.3.8 (excluding 240), do not pass in the .7z file when calling across Go contracts
		if (contract.RuntimeType == common.RuntimeType_DOCKER_GO || contract.RuntimeType == common.RuntimeType_GO) &&
			c.Ctx.GetBlockVersion() >= params.V2030800 && c.Ctx.GetBlockVersion() != params.V2040000 {
			res, _, stat = c.Ctx.CallContract(caller, contract, method, nil, parameters, gasUsed, common.TxType_INVOKE_CONTRACT)
		} else {
			res, _, stat = c.Ctx.CallContract(caller, contract, method, byteCode, parameters, gasUsed, common.TxType_INVOKE_CONTRACT)
		}
		c.SystemLog.DebugDynamic(func() string {
			return fmt.Sprintf("cross vm call contract %s", contract.String())
		})
	}

	c.OutParams.Reset()
	return res, stat
}
