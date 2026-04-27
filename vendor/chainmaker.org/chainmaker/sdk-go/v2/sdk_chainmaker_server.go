/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chainmaker_sdk_go

import (
	"context"
	"fmt"

	"chainmaker.org/chainmaker/pb-go/v2/config"
)

// GetChainMakerServerVersion get chainmaker version
func (cc *ChainClient) GetChainMakerServerVersion() (string, error) {
	cc.logger.Debug("[SDK] begin to get chainmaker server version")
	req := &config.ChainMakerVersionRequest{}
	client, err := cc.pool.getClient()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	res, err := client.rpcNode.GetChainMakerVersion(ctx, req)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", fmt.Errorf("get chainmaker server version failed, %s", res.Message)
	}
	return res.Version, nil
}

// GetChainMakerServerVersionCustom get chainmaker version
func (cc *ChainClient) GetChainMakerServerVersionCustom(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cc.logger.Debug("[SDK] begin to get chainmaker server version")
	req := &config.ChainMakerVersionRequest{}
	client, err := cc.pool.getClient()
	if err != nil {
		return "", err
	}
	res, err := client.rpcNode.GetChainMakerVersion(ctx, req)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", fmt.Errorf("get chainmaker server version failed, %s", res.Message)
	}
	return res.Version, nil
}
