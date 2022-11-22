/*
   Created by guoxin in 2022/11/17 9:37 AM
*/
package id

const (
	// TODO 常量命令
	// 编码前缀
	TaxationAdministrationCode string = "0"
	TaxationCode               string = "1"
	GovernmentBodyCode         string = "2"
	SocialCreditCodeCode       string = "3"
	IDCardNumberCode           string = "4"

	// 标识符长度
	TaxationAdministrationLength = 11
	TaxationLength               = 11
	GovernmentBodyLength         = 19
	SocialCreditCodeLength       = 19
	IDCardNumberLength           = 19

	Both = "00"

	NilString = ""

	// ValidTaxationCodeIndex 该常量用于描述税务编号（TaxationCode）和身份证编号（IDCardNumberCode）
	ValidTaxationCodeIndex = 1
	// ValidExternalCodeIndex 该常量用于描述税务编号（GovernmentBodyCode）和身份证编号（SocialCreditCodeCode）
	ValidExternalCodeIndex = 3
	// NotFoundIndex 如果在id中没有匹配则使用-1标示
	NotFoundIndex = -1
)
