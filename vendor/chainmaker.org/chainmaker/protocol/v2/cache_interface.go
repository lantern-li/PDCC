/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package protocol is a protocol package, which is base.
package protocol

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
)

type ProposalData struct {
	// block
	Block *common.Block
	// rw set Map
	TxRwSetMap map[string]*common.TxRWSet
	// contract event map
	ContractEventMap map[string][]*common.ContractEvent
	// additional data of proposal data
	AdditionalData map[string][]byte
}

// ProposalCache Cache proposed blocks that are not committed yet
type ProposalCache interface {
	// ClearProposedBlockAt Clear proposed blocks with height.
	ClearProposedBlockAt(height uint64)
	// GetProposedBlocksAt Get all proposed blocks at a specific height
	GetProposedBlocksAt(height uint64) []*common.Block
	// GetProposedBlock Get proposed block with specific block hash in current consensus height.
	GetProposedBlock(b *common.Block) *ProposalData
	// SetProposedBlock Set proposed block in current consensus height, after it's generated or verified.
	SetProposedBlock(proposalData *ProposalData, selfProposed bool) error
	// GetSelfProposedBlockAt Get proposed block that is proposed by node itself.
	GetSelfProposedBlockAt(height uint64) *common.Block
	// GetProposedBlockByHashAndHeight Get proposed block by block hash and block height
	GetProposedBlockByHashAndHeight(hash []byte, height uint64) (*common.Block, map[string]*common.TxRWSet)
	// HasProposedBlockAt Return if a proposed block has cached in current consensus height.
	HasProposedBlockAt(height uint64) bool
	// IsProposedAt Return if this node has proposed a block as proposer.
	IsProposedAt(height uint64) bool
	// SetProposedAt To mark this node has proposed a block as proposer.
	SetProposedAt(height uint64)
	// ResetProposedAt Reset propose status of this node.
	ResetProposedAt(height uint64)
	// KeepProposedBlock Remove proposed block in height except the specific block.
	KeepProposedBlock(hash []byte, height uint64) []*common.Block
	// DiscardBlocks Delete blocks data greater than the baseHeight
	DiscardBlocks(baseHeight uint64) []*common.Block
	// ClearTheBlock clean the special block in proposerCache
	ClearTheBlock(block *common.Block)
}

// LedgerCache Cache the latest block in ledger(DB).
type LedgerCache interface {
	// GetLastCommittedBlock Get the latest committed block
	GetLastCommittedBlock() *common.Block
	// SetLastCommittedBlock Set the latest committed block
	SetLastCommittedBlock(b *common.Block)
	// CurrentHeight Return current block height
	CurrentHeight() (uint64, error)
}
