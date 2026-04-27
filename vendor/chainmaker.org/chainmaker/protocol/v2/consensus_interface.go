/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package protocol is a protocol package, which is base.
package protocol

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
)

// TBFT chain config keys
const (
	TBFT_propose_timeout_key       = "TBFT_propose_timeout"
	TBFT_propose_delta_timeout_key = "TBFT_propose_delta_timeout"
	TBFT_blocks_per_proposer       = "TBFT_blocks_per_proposer"
)

const (
	// MLCMark specifies the multi layered consensus configurations mark
	MLCMark = "MLC"

	// MLCNMark specifies the multi layered consensus node mark
	MLCNMark = "MLCN"

	// MKCRTMark specifies the multi layered consensus rule type mark
	MKCRTMark = "MLCRT"

	// MLCRCMark specifies the multi layered consensus content mark
	MLCRCMark = "MLCRC"

	// MLCAdminMark specifies the multi layered consensus admin
	MLCAdminMark = "MLCAdmin"

	// MLCRTContractNameInPayload MLCRT based on matching the contract name in the transaction payload
	MLCRTContractNameInPayload = "0"

	// MLCRTKeyValueInPayload MLCRT based on matching the key and value in the transaction payload parameters
	// (e.g., "key: keyReg && value: valueReg")
	MLCRTKeyValueInPayload = "1"

	// MLCRTSenderCertificate MLCRT based on exact matching of the transaction sender's certificate/public key
	MLCRTSenderCertificate = "2"

	// MLCRTSenderOrgId MLCRT based on matching the transaction sender's orgId
	MLCRTSenderOrgId = "3"

	// MLCRTContractNameInReadSet MLCRT based on matching the contract name in the transaction's read set
	MLCRTContractNameInReadSet = "4"

	// MLCRTKeyValueInReadSet MLCRT based on matching the key and value in the transaction's read set parameters
	// (e.g., "key: keyReg && value: valueReg")
	MLCRTKeyValueInReadSet = "5"

	// MLCRTContractNameInWriteSet MLCRT based on matching the contract name in the transaction's write set
	MLCRTContractNameInWriteSet = "6"

	// MLCRTKeyValueInWriteSet MLCRT based on matching the key and value in the transaction's write set parameters
	// (e.g., "key: keyReg && value: valueReg")
	MLCRTKeyValueInWriteSet = "7"

	// MLCRTEventTopicAndPayload MLCRT based on matching the event topic and payload
	// (e.g., "(topic: topicReg) &&( payload: payloadReg)")
	MLCRTEventTopicAndPayload = "8"
)

// ConsensusEngine consensus abstract engine
type ConsensusEngine interface {
	// Start the consensus engine.
	Start() error
	// Stop stops the consensus engine.
	Stop() error
	// ConsensusState get the consensus state
	ConsensusState
}

// ConsensusState get consensus state
type ConsensusState interface {
	GetValidators() ([]string, error)
	GetLastHeight() uint64
	GetConsensusStateJSON() ([]byte, error)
}

// ConsensusExtendEngine extend engine for consensus
type ConsensusExtendEngine interface {
	ConsensusEngine
	InitExtendHandler(handler ConsensusExtendHandler)
}

// ConsensusExtendHandler extend consensus handler
type ConsensusExtendHandler interface {
	// CreateRWSet Creates a RwSet for the proposed block
	CreateRWSet(preBlkHash []byte, proposedBlock *consensuspb.ProposalBlock) error
	// VerifyConsensusArgs Verify the contents of the DPoS RwSet contained within the block
	VerifyConsensusArgs(block *common.Block, blockTxRwSet map[string]*common.TxRWSet) error
	// GetValidators Gets the validators for the current epoch
	GetValidators() ([]string, error)
}
