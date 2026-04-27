package endorsement

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"fmt"
)

type Endorsement struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	endorsementContractName = syscontract.SystemContract_ENDORSEMENT.String()
)

func NewEndorsement(log protocol.Logger) *Endorsement {
	return &Endorsement{
		log:     log,
		methods: registerEndorsementContractMethods(log),
	}
}

func (e *Endorsement) GetMethod(methodName string) common.ContractFunc {
	return e.methods[methodName]
}

func registerEndorsementContractMethods(log protocol.Logger) map[string]common.ContractFunc {
	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &Runtime{log: log}
	methodMap[syscontract.Endorsement_Endorse.String()] = common.WrapResultFunc(runtime.Endorse)
	methodMap[syscontract.Endorsement_GetEndorsementByHash.String()] = common.WrapResultFunc(runtime.GetEndorsementByHash)
	methodMap[syscontract.Endorsement_GetEndorsementByHeight.String()] = common.WrapResultFunc(runtime.GetEndorsementByHeight)

	return methodMap
}

// Runtime instance
type Runtime struct {
	log protocol.Logger
}

const (
	CHAINID = "chainId"
	HEIGHT  = "height"
	HASH    = "hash"
	DATA    = "data"
)

// Endorse the block header
func (r *Runtime) Endorse(context protocol.TxSimContext, parameters map[string][]byte) ([]byte, error) {

	chainId, height, hash, data, err := validateParameters(parameters, syscontract.Endorsement_Endorse.String())
	if err != nil {
		return nil, err
	}

	// key1 => 存储区块头的键值
	key1 := generateKeyWithHeight(chainId, height)

	// key2 => 存储高度与hash的对应关系
	key2 := generateKeyWithHash(chainId, hash)

	if err = context.Put(endorsementContractName, []byte(key2), parameters[HEIGHT]); err != nil {
		return nil, err
	}

	return nil, context.Put(endorsementContractName, []byte(key1), data)
}

func (r *Runtime) GetEndorsementByHash(context protocol.TxSimContext, parameters map[string][]byte) ([]byte, error) {

	chainId, _, hash, _, err := validateParameters(parameters, syscontract.Endorsement_GetEndorsementByHash.String())
	if err != nil {
		return nil, err
	}

	// 根据hash查到对应高度
	key1 := generateKeyWithHash(chainId, hash)
	height, err := context.Get(endorsementContractName, []byte(key1))
	if err != nil {
		return nil, err
	}

	ht := string(height)
	if height == nil || ht == "" || ht <= "0" {
		return nil, nil
	}

	key2 := generateKeyWithHeight(chainId, string(height))

	return context.Get(endorsementContractName, []byte(key2))
}

func (r *Runtime) GetEndorsementByHeight(context protocol.TxSimContext, parameters map[string][]byte) ([]byte, error) {

	chainId, height, _, _, err := validateParameters(parameters, syscontract.Endorsement_GetEndorsementByHeight.String())
	if err != nil {
		return nil, err
	}

	key := generateKeyWithHeight(chainId, height)

	return context.Get(endorsementContractName, []byte(key))
}

func validateParameters(parameters map[string][]byte, funcName string) (string, string, string, []byte, error) {

	switch funcName {
	case syscontract.Endorsement_Endorse.String():
		return validateEndorseParameters(parameters)
	case syscontract.Endorsement_GetEndorsementByHash.String():
		return validateGetEndorsementByHashParameters(parameters)
	case syscontract.Endorsement_GetEndorsementByHeight.String():
		return validateGetEndorsementByHeightParameters(parameters)
	default:
		return "", "", "", nil, fmt.Errorf("invaild method")
	}
}

func validateEndorseParameters(parameters map[string][]byte) (string, string, string, []byte, error) {
	chainId := parameters[CHAINID]
	height := parameters[HEIGHT]
	hash := parameters[HASH]
	data := parameters[DATA]

	if chainId == nil || height == nil || hash == nil || data == nil {
		return "", "", "", nil, fmt.Errorf("validate parameters fail, parameters could not be nil")
	}

	cId := string(chainId)
	if cId == "" {
		return "", "", "", nil, fmt.Errorf("validate height fail,chainId:%s", cId)
	}

	ht := string(height)
	if ht == "" || ht < "0" {
		return cId, ht, "", nil, fmt.Errorf("validate height fail,chainId:%s, height:%s", cId, ht)
	}

	hs := string(hash)
	if hs == "" {
		return "", "", "", nil, fmt.Errorf("validate hash fail,chainId:%s, height:%s,hash:%s", cId, ht, hs)
	}

	return cId, ht, hs, data, nil
}

func validateGetEndorsementByHashParameters(parameters map[string][]byte) (string, string, string, []byte, error) {
	chainId := parameters[CHAINID]
	hash := parameters[HASH]

	if chainId == nil || hash == nil {
		return "", "", "", nil, fmt.Errorf("validate parameters fail, parameters could not be nil")
	}

	cId := string(chainId)
	if cId == "" {
		return "", "", "", nil, fmt.Errorf("validate height fail,chainId:%s", cId)
	}

	hs := string(hash)
	if hs == "" {
		return "", "", "", nil, fmt.Errorf("validate hash fail,chainId:%s,hash:%s", cId, hs)
	}

	return cId, "", hs, nil, nil
}

func validateGetEndorsementByHeightParameters(parameters map[string][]byte) (string, string, string, []byte, error) {
	chainId := parameters[CHAINID]
	height := parameters[HEIGHT]

	if chainId == nil || height == nil {
		return "", "", "", nil, fmt.Errorf("validate parameters fail, parameters could not be nil")
	}

	cId := string(chainId)
	if cId == "" {
		return "", "", "", nil, fmt.Errorf("validate height fail,chainId:%s", cId)
	}

	ht := string(height)
	if ht == "" || ht < "0" {
		return cId, ht, "", nil, fmt.Errorf("validate height fail,chainId:%s, height:%s", cId, ht)
	}

	return cId, "", "", nil, nil
}

func generateKeyWithHeight(chainId, height string) string {
	return chainId + "-" + height
}

func generateKeyWithHash(chainId, hash string) string {
	return chainId + "-" + hash
}
