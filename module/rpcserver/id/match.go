/*
   Created by guoxin in 2022/11/17 9:24 AM
*/
package id

import (
	"chainmaker.org/chainmaker/protocol/v2"
	"fmt"
)

var IdentityMatchInstance = IdentityMatchImpl{}

type IdentityMatch interface {
	Match(subscriberId string, participantId ParticipantId) bool
}

type IdentityMatchImpl struct {
	log protocol.Logger
}

func InitIdentityMatch(log protocol.Logger) {
	IdentityMatchInstance = IdentityMatchImpl{log: log}
}
func (m IdentityMatchImpl) Match(subscriberId SubscriberId, participantId ParticipantId) bool {
	subscriberSubId, offset := subscriberId.GetIdentity()
	participantSubId := participantId.GetIdentity(subscriberId.GetType(), offset)
	if subscriberSubId == participantSubId {
		m.log.DebugDynamic(func() string {
			return fmt.Sprintf("identity match, subscriberId:%v,participantId:%v,subscriberSubId:%v,offset:%v,"+
				"participantSubId:%v", subscriberId.GetId(), participantId, subscriberSubId, offset, participantSubId)
		})
		return true
	} else {
		m.log.DebugDynamic(func() string {
			return fmt.Sprintf("identity mismatching, subscriberId:%v,participantId:%v,subscriberSubId:%v,offset:%v,"+
				"participantSubId:%v", subscriberId.GetId(), participantId, subscriberSubId, offset, participantSubId)
		})
		return false
	}
}
