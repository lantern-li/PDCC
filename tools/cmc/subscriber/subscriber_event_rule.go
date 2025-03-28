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

// NewSubEventWithRuleCMD new sub block with rule
func newSubEventWithRuleCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-with-rule",
		Short: "subscribe real-time/history event with rule",
		Long:  "subscribe real-time/history event with rule",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return subscriberEventByRule()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
		flagStartBlock,
		flagEndBlock,
		flagRuleType,
		flagContractName,
		flagMethod,
		flagTopic,
	})
	return cmd
}

func subscriberEventByRule() error {
	client, err := examples.CreateChainClientWithSDKConf(sdkConfPath)
	if err != nil {
		fmt.Println("read sdk config failed, err:", err)
		return err
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c, err := client.SubscribeContractEventWithRuleType(ctx, startBlock, endBlock, contractName, topic,
			method, txassign.RuleType(ruleType))
		if err != nil {
			fmt.Println("Subscribe event failed, err:", err)
			return
		}

		for {
			select {
			case event, ok := <-c:
				if !ok {
					fmt.Println("chan is close!")
				}
				if event == nil {
					fmt.Println("got nil event")
				}
				contractInfo, ok := event.(*common.ContractEventInfo)
				if !ok {
					fmt.Printf("event convert error: %v\n", err.Error())
				} else {
					fmt.Printf("recv contract event [%v, %v] => %v\n", contractName, contractInfo.BlockHeight, contractInfo)
				}
			case <-ctx.Done():
				fmt.Println("subscribe event ctx done")
			}
		}
	}()
	select {}
}
