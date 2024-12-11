/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
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

	DoubleZero = "00"

	NilString = ""

	// ValidTaxAndIdIndex 该常量用于描述 省局（TaxationCode）和身份证编号（IDCardNumberCode）
	ValidTaxAndIdIndex = 1
	// ValidGovAndEnpIndex 该常量用于描述 外部政府（GovernmentBodyCode）和企业（SocialCreditCodeCode）
	ValidGovAndEnpIndex = 3
	// NotFoundIndex 如果在id中没有匹配则使用-1标示
	NotFoundIndex = -1
)
