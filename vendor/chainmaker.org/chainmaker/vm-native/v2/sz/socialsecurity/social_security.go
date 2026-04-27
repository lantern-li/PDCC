package socialsecurity

import (
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/vm-native/v2/common"
	"chainmaker.org/chainmaker/vm-native/v2/sz"
)

type SocialSecurity struct {
	methods map[string]common.ContractFunc
	log     protocol.Logger
}

var (
	socialSecurityContractName = syscontract.SystemContract_SOCIAL_SECURITY.String()
)

func NewSocialSecurity(log protocol.Logger) *SocialSecurity {
	return &SocialSecurity{
		log:     log,
		methods: registerSocialSecurityMethods(log),
	}
}

func (c *SocialSecurity) GetMethod(methodName string) common.ContractFunc {
	return c.methods[methodName]
}

func registerSocialSecurityMethods(log protocol.Logger) map[string]common.ContractFunc {

	methodMap := make(map[string]common.ContractFunc, 64)
	runtime := &sz.Runtime{
		Log:          log,
		ContractName: socialSecurityContractName,
	}

	methodMap[syscontract.SocialSecurity_Save.String()] = common.WrapResultFunc(runtime.Save)
	methodMap[syscontract.SocialSecurity_Update.String()] = common.WrapResultFunc(runtime.Update)
	methodMap[syscontract.SocialSecurity_Get.String()] = common.WrapResultFunc(runtime.Get)
	methodMap[syscontract.SocialSecurity_TraceSource.String()] =
		common.WrapResultFunc(runtime.TraceSource)

	return methodMap
}
