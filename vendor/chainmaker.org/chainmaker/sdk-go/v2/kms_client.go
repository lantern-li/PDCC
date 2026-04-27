package chainmaker_sdk_go

import (
	"fmt"

	"github.com/spf13/viper"

	"chainmaker.org/chainmaker/common/v2/crypto/szkms"
	"chainmaker.org/chainmaker/sdk-go/v2/utils"
)

// SZKmsSource type kms source
type SZKmsSource string

const (
	// Bwmmfw type Bwmmfw
	Bwmmfw SZKmsSource = "bwmmfw"
	// Plugin type pligin
	Plugin SZKmsSource = "plugin"
	// Tencentcloudkms type tencentcloud
	Tencentcloudkms SZKmsSource = "tencentcloudkms"
)

// SZKMSClient kms interface
type SZKMSClient interface {
	Encrypt(plainData []byte) ([]byte, error)
	Decrypt(cipherData []byte) ([]byte, error)
}

// NewSZKMSClient create kms client instance
func NewSZKMSClient(opts ...SZKMSOption) (SZKMSClient, error) {
	config, err := generateKMSConfig(opts...)
	if err != nil {
		return nil, err
	}
	switch config.SZKMSBasicConfig.Source {
	case string(Bwmmfw):
		client := &BWMMFWKMSClient{
			Config: &szkms.KMSConfig{
				SecretId:      config.SZKMSBasicConfig.SecretID,
				SecretKey:     config.SZKMSBasicConfig.SecretKey,
				ServerAddress: config.SZKMSBasicConfig.Address,
				LibraryPath:   config.SZKMSBasicConfig.Library,
			},
			log: config.logger,
		}
		if err := client.init(); err != nil {
			return nil, err
		}
		return client, nil
	case string(Tencentcloudkms):
		// todo implement
		return nil, nil
	case string(Plugin):
		// todo implement
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported KMS source: %s", config.SZKMSBasicConfig.Source)
	}
}

// SZKMSConfig KMS配置
type SZKMSConfig struct {
	logger   utils.Logger
	confPath string

	// detail
	SZKMSBasicConfig *SZKMSBasicConfig `mapstructure:"kms"`
}

// SZKMSBasicConfig KMS基础配置
type SZKMSBasicConfig struct {
	KMSID     string `mapstructure:"kms_id"`
	Source    string `mapstructure:"source"`
	SecretID  string `mapstructure:"secret_id"`
	SecretKey string `mapstructure:"secret_key"`
	Address   string `mapstructure:"address"`
	Library   string `mapstructure:"library"`
	IsPublic  bool   `mapstructure:"is_public"`
	Region    string `mapstructure:"region"`
	SDKScheme string `mapstructure:"sdk_scheme"`
	ExtParams string `mapstructure:"ext_params"`
}

// SZKMSOption define KMS option func
type SZKMSOption func(config *SZKMSConfig)

// generateKMSConfig 读取kms配置
func generateKMSConfig(opts ...SZKMSOption) (*SZKMSConfig, error) {
	config := &SZKMSConfig{}
	for _, opt := range opts {
		opt(config)
	}

	if err := readKMSConfigFile(config); err != nil {
		return nil, fmt.Errorf("read kms config file failed, %v", err.Error())
	}
	if config.logger == nil {
		config.logger = getDefaultLogger()
	}
	return config, nil
}

// readKMSConfigFile 读取kms配置
func readKMSConfigFile(config *SZKMSConfig) error {
	if config.confPath == "" {
		return nil
	}
	var (
		configModel *SZKMSBasicConfig
		err         error
	)
	if configModel, err = initConfig(config.confPath); err != nil {
		return fmt.Errorf("init config failed, %s", err.Error())
	}

	config.SZKMSBasicConfig = configModel
	return nil
}

// initConfig 读取kms配置
func initConfig(confPath string) (*SZKMSBasicConfig, error) {
	var (
		err       error
		confViper *viper.Viper
	)

	if confViper, err = initViper(confPath); err != nil {
		return nil, fmt.Errorf("load kms config failed, %v", err)
	}

	configModel := &SZKMSConfig{
		SZKMSBasicConfig: &SZKMSBasicConfig{},
	}

	if err = confViper.Unmarshal(&configModel); err != nil {
		return nil, fmt.Errorf("unmarshal kms file failed, %v", err)
	}

	return configModel.SZKMSBasicConfig, nil
}

func initViper(confPath string) (*viper.Viper, error) {
	cmViper := viper.New()
	cmViper.SetConfigFile(confPath)
	if err := cmViper.ReadInConfig(); err != nil {
		return nil, err
	}

	return cmViper, nil
}

// WithKMSConfPath 设置配置文件路径
func WithKMSConfPath(confPath string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.confPath = confPath
	}
}

// WithKMSLogger 配置日志
func WithKMSLogger(logger utils.Logger) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.logger = logger
	}
}

// WithKMSId kmsId
func WithKMSId(kmsId string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.KMSID = kmsId
	}
}

// WithKMSSource kms source
func WithKMSSource(source string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.Source = source
	}
}

// WithKMSSecretID kms secret id
func WithKMSSecretID(secretId string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.SecretID = secretId
	}
}

// WithKMSSecretKey kms secret key
func WithKMSSecretKey(secretKey string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.SecretKey = secretKey
	}
}

// WithKMSAddress kms secret address
func WithKMSAddress(address string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.Address = address
	}
}

// WithKMSLibrary kms secret library
func WithKMSLibrary(library string) SZKMSOption {
	return func(config *SZKMSConfig) {
		config.SZKMSBasicConfig.Library = library
	}
}
