/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package chainmaker_sdk_go sdk
package chainmaker_sdk_go

import (
	"chainmaker.org/chainmaker/common/v2/crypto/szkms"
	"chainmaker.org/chainmaker/sdk-go/v2/utils"
)

// BWMMFWKMSClient BWMMFW client
type BWMMFWKMSClient struct {
	Config *szkms.KMSConfig
	client *szkms.SzKMSClient
	log    utils.Logger
}

// init init client
func (c *BWMMFWKMSClient) init() error {
	client, err := szkms.NewSingletonSzKMS(c.Config)
	if err != nil {
		return err
	}
	c.client = client
	return nil
}

// Encrypt encrypt data
func (c *BWMMFWKMSClient) Encrypt(data []byte) ([]byte, error) {
	c.log.Infof("BWMMFWKMSClient encrypt [%v]", data)
	return c.client.EncryptData(data)
}

// Decrypt decrypt data
func (c *BWMMFWKMSClient) Decrypt(data []byte) ([]byte, error) {
	c.log.Info("BWMMFWKMSClient Decrypt [%v]", data)
	return c.client.DecryptData(data)
}
