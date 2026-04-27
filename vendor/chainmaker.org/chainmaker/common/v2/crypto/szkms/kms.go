package szkms

import (
	"fmt"
	"github.com/avast/retry-go"
	"plugin"
	"sync"
	"time"
)

const (
	InitSysFunc     = "InitSys"
	EncryptDataFunc = "EncryptData"
	DecryptDataFunc = "DecryptData"
	SignDataFunc    = "SignData"

	DefaultRetryLimit    = 10
	DefaultRetryInterval = 2
)

type KMSConfig struct {
	SecretId      string
	SecretKey     string
	ServerAddress string
	LibraryPath   string
}

type SzKMSClient struct {
	config              *KMSConfig
	instance            *plugin.Plugin
	initSysInstance     plugin.Symbol
	encryptDataInstance plugin.Symbol
	decryptDataInstance plugin.Symbol
	signDataInstance    plugin.Symbol
}

var instances sync.Map

func NewSingletonSzKMS(config *KMSConfig) (*SzKMSClient, error) {
	if client, ok := instances.Load(config.SecretId); ok {
		return client.(*SzKMSClient), nil
	}
	ins, err := newSzKMS(config)
	if err != nil {
		return nil, err
	}
	if err := ins.InitSys(); err != nil {
		return nil, fmt.Errorf("[id: %v] failed to initialize KMS system: %v", config.SecretId, err)
	}
	instances.Store(config.SecretId, ins)
	return ins, nil
}

func NewSzKMS(config *KMSConfig) (*SzKMSClient, error) {
	ins, err := newSzKMS(config)
	if err != nil {
		return nil, err
	}
	if err := ins.InitSys(); err != nil {
		return nil, fmt.Errorf("[id: %v] failed to initialize KMS system: %v", config.SecretId, err)
	}
	return ins, nil
}

func newSzKMS(config *KMSConfig) (*SzKMSClient, error) {
	ins, err := plugin.Open(config.LibraryPath)
	if err != nil {
		return nil, fmt.Errorf("plugin open lib fail. lib: %v, error: %v", config.LibraryPath, err)
	}
	getSymbol := func(name string) (plugin.Symbol, error) {
		sym, err := ins.Lookup(name)
		if err != nil {
			return nil, fmt.Errorf("plugin lookup [%v] function fail. error: %v", name, err)
		}
		return sym, nil
	}
	initSysFnIns, err := getSymbol(InitSysFunc)
	if err != nil {
		return nil, err
	}
	encryptDataIns, err := getSymbol(EncryptDataFunc)
	if err != nil {
		return nil, err
	}
	decryptDataIns, err := getSymbol(DecryptDataFunc)
	if err != nil {
		return nil, err
	}
	signDataIns, err := getSymbol(SignDataFunc)
	if err != nil {
		return nil, err
	}

	return &SzKMSClient{config: config, instance: ins, initSysInstance: initSysFnIns,
		encryptDataInstance: encryptDataIns, decryptDataInstance: decryptDataIns,
		signDataInstance: signDataIns}, nil
}

func (s *SzKMSClient) InitSys() error {
	return retry.Do(
		func() error {
			if err := s.initSys(); err != nil {
				return err
			}
			fmt.Println("init sys success!")
			return nil
		},
		retry.Attempts(uint(DefaultRetryLimit)),                          // 最大重试次数
		retry.MaxJitter(time.Second*time.Duration(DefaultRetryInterval)), // 随机最大等待时间
		retry.OnRetry(func(n uint, err error) {
			fmt.Printf("[try: %v] init sys failed: %v\n", n+1, err)
		}),
	)
}

func (s *SzKMSClient) initSys() error {
	ok, msg := s.initSysInstance.(func(string, string) (bool, string))(s.config.SecretId, s.config.ServerAddress)
	if !ok {
		return fmt.Errorf("invoking InitSys function fail. error: %v", msg)
	}
	return nil
}

func (s *SzKMSClient) EncryptData(plainData []byte) ([]byte, error) {
	result, err := s.encryptDataInstance.(func([]byte) ([]byte, error))(plainData)
	if err != nil {
		return nil, fmt.Errorf("invoking EncryptData function fail. error: %v", err.Error())
	}
	return result, nil
}

func (s *SzKMSClient) DecryptData(cipherData []byte) ([]byte, error) {
	result, err := s.decryptDataInstance.(func([]byte) ([]byte, error))(cipherData)
	if err != nil {
		return nil, fmt.Errorf("invoking DecryptData function fail. error: %v", err.Error())
	}
	return result, nil
}

func (s *SzKMSClient) SignData(digest []byte, keyId string) (sign string, err error) {
	ok, msg := s.signDataInstance.(func([]byte, string) (bool, string))(digest, keyId)
	if !ok {
		return "", fmt.Errorf("invoking SignData function fail. key id: %v, error: %v", keyId, msg)
	}
	return msg, nil
}
