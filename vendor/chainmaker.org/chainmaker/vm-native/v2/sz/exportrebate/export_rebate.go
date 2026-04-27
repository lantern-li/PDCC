package exportrebate

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"chainmaker.org/chainmaker/vm-native/v2/sz"
)

type ExportRebate struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	exportRebateContractName = syscontract.SystemContract_EXPORT_REBATE.String()
)

func NewExportRebate(log protocol.Logger) *ExportRebate {
	return &ExportRebate{
		log:     log,
		methods: registerExportRebateMethods(log),
	}
}

func (c *ExportRebate) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

func registerExportRebateMethods(log protocol.Logger) map[string]common.ContractFunc {

	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &sz.Runtime{
		Log:          log,
		ContractName: exportRebateContractName,
	}

	methodMap[syscontract.ExportRebate_Save.String()] = common.WrapResultFunc(runtime.Save)
	methodMap[syscontract.ExportRebate_Update.String()] = common.WrapResultFunc(runtime.Update)
	methodMap[syscontract.ExportRebate_Get.String()] = common.WrapResultFunc(runtime.Get)
	methodMap[syscontract.ExportRebate_TraceSource.String()] =
		common.WrapResultFunc(runtime.TraceSource)

	return methodMap
}
