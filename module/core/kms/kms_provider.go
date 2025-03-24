package kms

import (
	"chainmaker.org/chainmaker/common/v2/crypto/szkms"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/protocol/v2"
	"fmt"
)

type KMSClient interface {
	protocol.KMSProvider
	init() error
}

type KmsSource string

const (
	Bwmmfw          KmsSource = "bwmmfw"
	Plugin          KmsSource = "plugin"
	Tencentcloudkms KmsSource = "tencentcloudkms "
)

func InitKMSProviders(kmsConfigs []localconf.KMSBasicConfig, log *logger.CMLogger) map[string]protocol.KMSProvider {
	kmsClients := make(map[string]protocol.KMSProvider)
	for _, config := range kmsConfigs {
		if !config.Enabled {
			continue
		}
		client, err := newKMSClient(config, log)
		if err != nil || client == nil {
			// 为了不影响链的正常启动，这里只打印错误信息，不返回错误
			log.Errorf("failed to init KMS (ID: %s): %v", config.KMSID, err)
		} else {
			log.Infof("init KMS provider [ID: %s] success", config.KMSID)
			kmsClients[config.KMSID] = client
		}
	}
	return kmsClients
}

// NewKMSClient 根据 source 创建对应的 KMS 实例
func newKMSClient(config localconf.KMSBasicConfig, log *logger.CMLogger) (KMSClient, error) {
	switch config.Source {
	case string(Bwmmfw):
		client := &BWMMFWKMSClient{
			Config: &szkms.KMSConfig{
				SecretId:      config.SecretID,
				SecretKey:     config.SecretKey,
				ServerAddress: config.Address,
				LibraryPath:   config.Library,
			},
			log: log,
		}
		if err := client.init(); err != nil {
			return nil, err
		}
		return client, nil
	case string(Tencentcloudkms):
		// todo implement
		return nil, fmt.Errorf("not implement")
	case string(Plugin):
		// todo implement
		return nil, fmt.Errorf("not implement")
	default:
		return nil, fmt.Errorf("unsupported KMS source: %s", config.Source)
	}
}
