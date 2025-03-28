// Copyright (C) BABEC. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package subscriber
package subscribe

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	sdkConfPath  string
	startBlock   int64
	endBlock     int64
	withRWSet    bool
	onlyHeader   bool
	ruleType     int32
	contractName string
	method       string
	txIds        []string
	topic        string
)

const (
	flagSdkConfPath  = "sdk-conf-path"
	flagStartBlock   = "start"
	flagEndBlock     = "end"
	flagWithRWSet    = "with-rwset"
	flagOnlyHeader   = "only-header"
	flagRuleType     = "rule-type"
	flagContractName = "contract-name"
	flagMethod       = "method"

	timeFormat1 = "2006-01-02 15:04:05.000"
	flagTopic   = "topic"
	flagTxIds   = "tx-ids"
)

var flags *pflag.FlagSet

func init() {
	flags = &pflag.FlagSet{}

	flags.StringVar(&sdkConfPath, flagSdkConfPath, "", "specify sdk config path")
	flags.Int64Var(&startBlock, flagStartBlock, -1, "specify subscriber start block height")
	flags.Int64Var(&endBlock, flagEndBlock, -1, "specify subscriber end block height")
	flags.BoolVar(&withRWSet, flagWithRWSet, false, "specify subscriber result with RWSet")
	flags.BoolVar(&onlyHeader, flagOnlyHeader, false, "specify subscriber result only header")
	flags.Int32Var(&ruleType, flagRuleType, 0, "specify subscriber rule type")
	flags.StringVar(&contractName, flagContractName, "T", "specify subscriber contract name")
	flags.StringVar(&method, flagMethod, "P", "specify subscriber method")
	flags.StringSliceVar(&txIds, flagTxIds, []string{}, "tx id list. --tx-ids=\"abc,xyz\"")
	flags.StringVar(&method, flagTopic, "", "specify subscriber topic")
}

// NewSubCMD new subscribe command
func NewSubCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "subscribe blockchain data",
		Long:  "subscribe blockchain data",
	}

	cmd.AddCommand(newSubBlockWithRuleCMD())
	cmd.AddCommand(newSubTxWithRuleCMD())
	cmd.AddCommand(newSubEventWithRuleCMD())

	return cmd
}
