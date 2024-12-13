package kms

import (
	"chainmaker.org/chainmaker/common/v2/crypto/szkms"
	"chainmaker.org/chainmaker/logger/v2"
)

type BWMMFWKMSClient struct {
	Config *szkms.KMSConfig
	client *szkms.SzKMSClient
	log    *logger.CMLogger
}

func (c *BWMMFWKMSClient) init() error {
	client, err := szkms.NewSingletonSzKMS(c.Config)
	if err != nil {
		return err
	}
	c.client = client
	return nil
}

func (c *BWMMFWKMSClient) Encrypt(data []byte) ([]byte, error) {
	c.log.Infof("BWMMFWKMSClient encrypt [%v]", data)
	return c.client.EncryptData(data)
}

func (c *BWMMFWKMSClient) Decrypt(data []byte) ([]byte, error) {
	c.log.Info("BWMMFWKMSClient Decrypt [%v]", data)
	return c.client.DecryptData(data)
}
