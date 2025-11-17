/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package verifier

import (
	"chainmaker.org/chainmaker-go/module/core/common"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
)

func parseVerifyResult(block *commonpb.Block, isValid bool,
	txsRwSet map[string]*commonpb.TxRWSet, rwSetVerifyFailTxs *common.RwSetVerifyFailTx) *consensuspb.VerifyResult {
	verifyResult := &consensuspb.VerifyResult{
		VerifiedBlock: block,
		TxsRwSet:      txsRwSet,
	}
	if isValid {
		verifyResult.Code = consensuspb.VerifyResult_SUCCESS
		verifyResult.Msg = "OK"
	} else {
		verifyResult.Msg = "FAIL"
		verifyResult.Code = consensuspb.VerifyResult_FAIL
		if rwSetVerifyFailTxs != nil {
			verifyResult.RwSetVerifyFailTxs = &consensuspb.RwSetVerifyFailTxs{
				TxIds:       rwSetVerifyFailTxs.TxIds,
				BlockHeight: rwSetVerifyFailTxs.BlockHeight,
			}
		}
	}
	return verifyResult
}
