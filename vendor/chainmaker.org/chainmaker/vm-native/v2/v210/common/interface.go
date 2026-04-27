/*
 * Copyright (C) BABEC. All rights reserved.
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package common210

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// ContractFunc comment at next version
type ContractFunc func(context protocol.TxSimContext, params map[string][]byte) *common.ContractResult

// Contract define native Contract interface
type Contract interface {
	GetMethod(methodName string) ContractFunc
}
