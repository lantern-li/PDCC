package receipt

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"chainmaker.org/chainmaker/vm-native/v2/sz"
)

type Receipt struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	receiptContractName = syscontract.SystemContract_RECEPIT.String()
)

func NewReceipt(log protocol.Logger) *Receipt {
	return &Receipt{
		log:     log,
		methods: registerReceiptMethods(log),
	}
}

func (c *Receipt) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

func registerReceiptMethods(log protocol.Logger) map[string]common.ContractFunc {

	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &sz.Runtime{
		Log:          log,
		ContractName: receiptContractName,
	}

	methodMap[syscontract.Receipt_Save.String()] = common.WrapResultFunc(runtime.Save)
	methodMap[syscontract.Receipt_Update.String()] = common.WrapResultFunc(runtime.Update)
	methodMap[syscontract.Receipt_Get.String()] = common.WrapResultFunc(runtime.Get)
	methodMap[syscontract.Receipt_TraceSource.String()] = common.WrapResultFunc(runtime.TraceSource)

	return methodMap
}
