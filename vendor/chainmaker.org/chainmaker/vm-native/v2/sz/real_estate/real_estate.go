package real_estate

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"chainmaker.org/chainmaker/vm-native/v2/sz"
)

type RealEstate struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	realEstateContractName = syscontract.SystemContract_REAL_ESTATE.String()
)

func NewRealEstate(log protocol.Logger) *RealEstate {
	return &RealEstate{
		log:     log,
		methods: registerRealEstateMethods(log),
	}
}

func (c *RealEstate) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

func registerRealEstateMethods(log protocol.Logger) map[string]common.ContractFunc {

	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &sz.Runtime{
		Log:          log,
		ContractName: realEstateContractName,
	}

	methodMap[syscontract.RealEstate_Save.String()] = common.WrapResultFunc(runtime.Save)
	methodMap[syscontract.RealEstate_Update.String()] = common.WrapResultFunc(runtime.Update)
	methodMap[syscontract.RealEstate_Get.String()] = common.WrapResultFunc(runtime.Get)
	methodMap[syscontract.RealEstate_TraceSource.String()] = common.WrapResultFunc(runtime.TraceSource)

	return methodMap
}
