/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package id

import (
	"errors"
)

type SubscriberId interface {
	GetType() string
	GetIdentity() (string, int)
	GetId() string
}

type SubscriberIdImpl struct {
	Id       string
	typ      string
	identity string
	offset   int
}

/*
	NewSubscriberId

总 		0 00 01 00 0000 X 不存在总局有区的情况
总 		0 00 00 00 0000 √
省	  	1 43 00 00 0000 √ -> 1 43
省市  	1 43 01 00 0000 √ -> 1 43 01
省市区 	1 43 01 11 0000 √ -> 1 43 01 11
其他（2 3 4）均全匹配
*/
func NewSubscriberId(id string) (SubscriberId, error) {
	if len(id) <= 2 {
		return nil, errors.New("subscriberId identifier is invalid, identifier length is less than 2")
	}

	subscriberId := &SubscriberIdImpl{
		Id:  id,
		typ: id[:1],
	}

	// 全匹配： 国家税务总局 0 外部政府机构 2 企业 3 自然人 4
	if subscriberId.typ == TaxationAdministrationCode ||
		subscriberId.typ == GovernmentBodyCode ||
		subscriberId.typ == SocialCreditCodeCode ||
		subscriberId.typ == IDCardNumberCode {
		subscriberId.identity = id
		subscriberId.offset = 0
		return subscriberId, nil
	}

	// 省市区匹配 00，找出对应的前缀
	var index int
	for i := 1; i < len(id)-2; i += 2 {
		if id[i:i+2] == DoubleZero {
			index = i
			break
		}
	}

	// 如果未匹配到00，则代表全有，则为 省市区情况
	// 如果 index > 7 ，则说明00在后四保留位，则为 省市区情况
	if index == 0 || index > 7 {
		index = 7
	}

	if index%2 == 0 {
		return nil, errors.New("subscriberId identifier is invalid, the identifier format is incorrect")
	}

	subscriberId.identity = id[ValidTaxAndIdIndex:index]
	subscriberId.offset = index - ValidTaxAndIdIndex

	return subscriberId, nil
}

func (s SubscriberIdImpl) GetType() string {
	return s.typ
}

func (s SubscriberIdImpl) GetIdentity() (string, int) {
	return s.identity, s.offset
}
func (s SubscriberIdImpl) GetId() string {
	return s.Id
}
