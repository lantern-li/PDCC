package kms

import (
	"chainmaker.org/chainmaker/localconf/v2"
	"testing"
)

func TestInitKMSProviders(t *testing.T) {
	configs := []localconf.KMSBasicConfig{
		{KMSID: "test1", Enabled: true, Source: "bwmmfw", SecretID: "", SecretKey: "", Address: "", Library: "../lib/kms/amd/bwmmfw.so"},
	}
	InitKMSProviders(configs, nil)
}
