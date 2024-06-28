/*
Created by guoxin in 2022/11/17 9:24 AM
*/
package id

import (
	"chainmaker.org/chainmaker/protocol/v2"
	"fmt"
	"strings"
)

var IdentityMatchInstance = IdentityMatchImpl{}

type IdentityMatchImpl struct {
	log protocol.Logger
}

func InitIdentityMatch(log protocol.Logger) {
	IdentityMatchInstance = IdentityMatchImpl{log: log}
}

/*
Match 用于匹配订阅者和参与者的身份
subscriberId: 订阅者身份
participantId: 参与方ID，取自链上交易

2 01 02 03 04 0000000000

1 10 02 03 04 0000000000
*/
func (m IdentityMatchImpl) Match(subscriberId SubscriberId, participantId string) bool {
	sType := subscriberId.GetType()
	switch sType {
	case TaxationAdministrationCode:
		return true
	case GovernmentBodyCode, SocialCreditCodeCode, IDCardNumberCode:
		return subscriberId.GetId() == participantId
	case TaxationCode: // 省局
		switch participantId[:1] {
		case TaxationAdministrationCode: // 总局
			return false
		case TaxationCode, IDCardNumberCode: // 省局 个人
			identity, _ := subscriberId.GetIdentity()                              // 取出省市区
			return strings.HasPrefix(participantId[ValidTaxAndIdIndex:], identity) // 前缀匹配
		case GovernmentBodyCode, SocialCreditCodeCode: // 外部政府 企业
			identity, _ := subscriberId.GetIdentity()
			return strings.HasPrefix(participantId[ValidGovAndEnpIndex:], identity)
		}
	}
	m.log.DebugDynamic(func() string {
		return fmt.Sprintf("identity mismatching, subscriberId:%v, participantId:%v", subscriberId.GetId(), participantId)
	})
	return false

}
