package wasmer

type VMConfig struct {
	// kms config
	KMSConfig KMSConfig `mapstructure:"kms"`
}

type KMSConfig struct {
	Enable bool   `mapstructure:"enable"`
	Id     string `mapstructure:"id"`
}
