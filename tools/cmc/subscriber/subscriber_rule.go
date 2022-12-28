// Copyright (C) BABEC. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package subscriber
package subscriber

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/sdk-go/v2/examples"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"time"
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
)

const (
	flagSdkConfPath  = "sdk-conf-path"
	flagStart        = "start"
	flagEnd          = "end"
	flagWithRWSet    = "with-rwset"
	flagOnlyHeader   = "only-header"
	flagRuleType     = "rule-type"
	flagContractName = "contract-name"
	flagMethod       = "method"

	timeFormat1 = "2006-01-02 15:04:05.000"
)

var flags *pflag.FlagSet

func init() {
	flags = &pflag.FlagSet{}

	flags.StringVar(&sdkConfPath, flagSdkConfPath, "", "specify sdk config path")
	flags.Int64Var(&startBlock, flagStart, -1, "specify subscriber end block height")
	flags.Int64Var(&endBlock, flagEnd, -1, "specify subscriber start block height")
	flags.BoolVar(&withRWSet, flagWithRWSet, false, "specify subscriber result with RWSet")
	flags.BoolVar(&onlyHeader, flagOnlyHeader, false, "specify subscriber result only header")
	flags.Int32Var(&ruleType, flagRuleType, 0, "specify subscriber rule type")
	flags.StringVar(&contractName, flagContractName, "T", "specify subscriber contract name")
	flags.StringVar(&method, flagMethod, "P", "specify subscriber method")
}

// VersionCMD show chainmaker client version
func TxAssignCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign",
		Short: "Show ChainMaker Client transaction assign",
		Long:  "Show ChainMaker Client transaction assign",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return subscriberByRule()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
		flagStart,
		flagEnd,
		flagWithRWSet,
		flagOnlyHeader,
		flagRuleType,
		flagContractName,
		flagMethod,
	})
	return cmd
}

func subscriberByRule() error {
	client, err := examples.CreateChainClientWithSDKConf(sdkConfPath)
	if err != nil {
		fmt.Println("read sdk config failed, err:", err)
		return err
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c, err := client.SubscribeBlockWithRule(ctx, startBlock, endBlock, withRWSet, onlyHeader, contractName, method,
			txassign.RuleType(ruleType))
		if err != nil {
			fmt.Println("Subscribe failed, err:", err)
			return
		}
		var total int
		for {
			select {
			case block, ok := <-c:
				if !ok {
					fmt.Println("chan is close!")
					return
				}

				if block == nil {
					fmt.Println("require not nil")
				}

				if onlyHeader {
					blockHeader, ok := block.(*common.BlockHeader)
					if !ok {
						fmt.Println("require true")
					}
					bytes, _ := json.Marshal(blockHeader)
					fmt.Printf("recv block header [%d] => %+v\n", blockHeader.BlockHeight, string(bytes))
				} else {
					blockInfo, ok := block.(*common.BlockInfo)
					if !ok {
						fmt.Println("require true")
					}
					for _, tx := range blockInfo.Block.Txs {
						bytes, _ := json.Marshal(tx)
						fmt.Printf("time:%s|recv block [%d] txs: %v, total: %v, txid: %v \n %v \n",
							time.Now().Format(timeFormat1), blockInfo.Block.Header.BlockHeight, len(blockInfo.Block.Txs), total, tx.Payload.TxId, string(bytes))
					}
				}
				time.Sleep(time.Second * 2)
				fmt.Println()

			case <-ctx.Done():
				return
			}
		}
	}()
	select {}
}
