/*
   Created by guoxin in 2022/11/17 9:34 AM
*/
package id

import (
	"errors"
	"fmt"
)

type ParticipantId interface {
	GetIdentity(subscriberType string, offset int) string
	Verify() error
}

func NewParticipantId(id string) (ParticipantId, error) {
	var participant ParticipantId
	switch string(id[0]) {
	case TaxationAdministrationCode:
		participant = TaxationAdministrationParticipant(id)
	case TaxationCode:
		participant = TaxationParticipant(id)
	case GovernmentBodyCode:
		participant = GovernmentBodyParticipant(id)
	case SocialCreditCodeCode:
		participant = SocialCreditCodeParticipant(id)
	case IDCardNumberCode:
		participant = IDCardNumberParticipant(id)
	default:
		return nil, errors.New("participant not support")
	}
	err := participant.Verify()
	if err != nil {
		return nil, err
	}
	return participant, nil
}

type TaxationAdministrationParticipant string

func (p TaxationAdministrationParticipant) Verify() error {
	if len(p) != TaxationAdministrationLength {
		return fmt.Errorf("TaxationAdministration length error, real: %v, want: %v, got: %v", p, TaxationAdministrationLength, len(p))
	}
	return nil
}

func (p TaxationAdministrationParticipant) GetIdentity(_ string, _ int) string {
	return string(p)
}

type TaxationParticipant string

func (p TaxationParticipant) Verify() error {
	if len(p) != TaxationLength {
		return fmt.Errorf("Taxation length error, real: %v, want: %v, got: %v", p, TaxationLength, len(p))
	}
	return nil
}

func (p TaxationParticipant) GetIdentity(subscriberType string, offset int) string {
	if subscriberType == SocialCreditCodeCode || subscriberType == GovernmentBodyCode {
		return NilString
	}
	return string(p)[ValidTaxationCodeIndex : ValidTaxationCodeIndex+offset]
}

type GovernmentBodyParticipant string

func (p GovernmentBodyParticipant) Verify() error {
	if len(p) != GovernmentBodyLength {
		return fmt.Errorf("GovernmentBody length error, real: %v, want: %v, got: %v", p, GovernmentBodyLength, len(p))
	}
	return nil
}

func (p GovernmentBodyParticipant) GetIdentity(subscriberType string, offset int) string {
	if subscriberType == SocialCreditCodeCode || subscriberType == GovernmentBodyCode {
		return string(p)
	}
	return string(p)[ValidExternalCodeIndex : ValidExternalCodeIndex+offset]
}

type SocialCreditCodeParticipant string

func (p SocialCreditCodeParticipant) Verify() error {
	if len(p) != SocialCreditCodeLength {
		return fmt.Errorf("SocialCreditCode length error, real: %v, want: %v, got: %v", p, SocialCreditCodeLength, len(p))
	}
	return nil
}

func (p SocialCreditCodeParticipant) GetIdentity(subscriberType string, offset int) string {
	if subscriberType == SocialCreditCodeCode || subscriberType == GovernmentBodyCode {
		return string(p)
	}
	return string(p)[ValidExternalCodeIndex : ValidExternalCodeIndex+offset]
}

type IDCardNumberParticipant string

func (p IDCardNumberParticipant) Verify() error {
	if len(p) != IDCardNumberLength {
		return fmt.Errorf("IDCardNumber length error, real: %v, want: %v, got: %v", p, IDCardNumberLength, len(p))
	}
	return nil
}

func (p IDCardNumberParticipant) GetIdentity(subscriberType string, offset int) string {
	if subscriberType == SocialCreditCodeCode || subscriberType == GovernmentBodyCode {
		return NilString
	}
	return string(p)[ValidTaxationCodeIndex : ValidTaxationCodeIndex+offset]
}
