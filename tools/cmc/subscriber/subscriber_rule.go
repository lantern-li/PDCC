// Copyright (C) BABEC. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package subscriber
package subscribe

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/common/v2/json"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/sdk-go/v2/examples"
	"context"
	"fmt"
	"github.com/spf13/cobra"
)

// NewSubBlockWithRuleCMD new sub block with rule
func newSubBlockWithRuleCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block-with-rule",
		Short: "subscribe real-time/history blocks with rule",
		Long:  "subscribe real-time/history blocks with rule",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return subscriberByRule()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
		flagStartBlock,
		flagEndBlock,
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
		for {
			select {
			case block, ok := <-c:
				if !ok {
					fmt.Println("chan is close!")
					return
				}

				if block == nil {
					fmt.Println("require not nil")
					continue
				}
				go func() {
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
							fmt.Printf("recv block [%d] => %+v\n", blockInfo.Block.Header.BlockHeight,
								tx.Payload)
						}

						//bytes, _ := json.Marshal(blockInfo.Block.Header)
						//fmt.Printf("recv block header [%d] => %+v\n", blockInfo.Block.Header.BlockHeight, string(bytes))
					}
				}()
			case <-ctx.Done():
				return
			}
		}
	}()
	select {}
}
