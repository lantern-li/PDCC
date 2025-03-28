// Copyright (C) BABEC. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package subscriber
package subscribe

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/sdk-go/v2/examples"
	"context"
	"fmt"
	"github.com/spf13/cobra"
)

// NewSubTxWithRuleCMD new sub tx with rule
func newSubTxWithRuleCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx-with-rule",
		Short: "subscribe real-time/history txs with rule",
		Long:  "subscribe real-time/history txs with rule",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return subscriberTxByRule()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
		flagStartBlock,
		flagEndBlock,
		flagRuleType,
		flagContractName,
		flagMethod,
		flagTxIds,
	})
	return cmd
}

func subscriberTxByRule() error {
	client, err := examples.CreateChainClientWithSDKConf(sdkConfPath)
	if err != nil {
		fmt.Println("read sdk config failed, err:", err)
		return err
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c, err := client.SubscribeTxWithRule(ctx, startBlock, endBlock, contractName, txIds,
			method, txassign.RuleType(ruleType))
		if err != nil {
			fmt.Println("Subscribe tx failed, err:", err)
			return
		}
		for {
			select {
			case tx, ok := <-c:
				if !ok {
					fmt.Println("chan is close!")
					return
				}
				if tx == nil {
					fmt.Println("tx is nil")
					continue
				}
				t := tx.(*common.Transaction)
				fmt.Printf("recv tx [%v] => %v \n", t.Payload.TxId, t.Payload)
			case <-ctx.Done():
				fmt.Println("ctx is done")
			}
		}
	}()
	select {}
}
