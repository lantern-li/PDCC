package protocol

// KMSProvider kms加解密接口
type KMSProvider interface {
	Encrypt(plainData []byte) ([]byte, error)
	Decrypt(cipherData []byte) ([]byte, error)
}
