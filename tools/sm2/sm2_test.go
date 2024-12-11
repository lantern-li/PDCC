/*
   Created by guoxin in 2022/12/2 4:08 PM
*/
package sm2

import (
	"chainmaker.org/chainmaker/common/v2/opencrypto/tencentsm/sm2"
	"crypto"
	"github.com/stretchr/testify/assert"
	tjsm2 "github.com/tjfoc/gmsm/sm2"
	"testing"
)

var msg = []byte("hello world")

func TestSignAndVerify(t *testing.T) {
	priv, err := sm2.GenerateKeyPair()
	sig, err := priv.ToStandardKey().(crypto.Signer).Sign(nil, msg, nil)
	assert.NoError(t, err)

	pub, ok := priv.PublicKey().ToStandardKey().(*tjsm2.PublicKey)
	assert.True(t, ok)
	assert.NotNil(t, pub)
	ok = pub.Verify(msg, sig)
	assert.True(t, ok)
	ok, err = priv.PublicKey().Verify(msg, sig)
	assert.NoError(t, err)
	assert.True(t, ok)
}
