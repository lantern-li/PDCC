/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper base test
package helper

import (
	"fmt"
	"testing"
)

func TestGetIdentityCode(t *testing.T) {
	ic, err := GetIdentityCode("14400000000")
	fmt.Println(ic, err)
	participant, err := GetIdentityCode("44420119700101XXXXX")
	fmt.Println(ic, err)
	b := ic.Match(participant)
	fmt.Println(b)
}
