/*
Created by guoxin in 2022/11/17 9:34 AM
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

func NewSubscriberId(id string) (SubscriberId, error) {
	subscriberId := &SubscriberIdImpl{
		Id:  id,
		typ: string(id[0]),
	}

	if subscriberId.typ == SocialCreditCodeCode || subscriberId.typ == GovernmentBodyCode || subscriberId.typ == TaxationAdministrationCode {
		subscriberId.identity = id
		subscriberId.offset = 0
		return subscriberId, nil
	}
	var index int
	for i := 1; i < len(id); i += 2 {
		if id[i:i+2] == Both {
			index = i
			break
		}
	}
	if index == 0 {
		return nil, errors.New("subscriberId identifier is invalid, identifier does not contain \"00\"")
	}

	if index%2 == 0 {
		return nil, errors.New("subscriberId identifier is invalid, the identifier format is incorrect")
	}
	subscriberId.identity = id[ValidTaxationCodeIndex:index]
	subscriberId.offset = index - ValidTaxationCodeIndex
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
