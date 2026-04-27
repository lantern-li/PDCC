package declaration

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"chainmaker.org/chainmaker/vm-native/v2/sz"
)

type Declaration struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	declarationContractName = syscontract.SystemContract_DECLARATION.String()
)

func NewDeclaration(log protocol.Logger) *Declaration {
	return &Declaration{
		log:     log,
		methods: registerDeclarationMethods(log),
	}
}

func (c *Declaration) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

func registerDeclarationMethods(log protocol.Logger) map[string]common.ContractFunc {

	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &sz.Runtime{
		Log:          log,
		ContractName: declarationContractName,
	}

	methodMap[syscontract.Declaration_Save.String()] = common.WrapResultFunc(runtime.Save)
	methodMap[syscontract.Declaration_Update.String()] = common.WrapResultFunc(runtime.Update)
	methodMap[syscontract.Declaration_Get.String()] = common.WrapResultFunc(runtime.Get)
	methodMap[syscontract.Declaration_TraceSource.String()] = common.WrapResultFunc(runtime.TraceSource)

	return methodMap
}
