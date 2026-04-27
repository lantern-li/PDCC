package utils

import (
	"sort"
	"strings"

	"chainmaker.org/chainmaker/protocol/v2"
)

const MLCSeparator = "-"

// buildKey is a helper function to construct keys with the given parts.
func buildKey(parts ...string) string {
	return strings.Join(parts, MLCSeparator)
}

// GetMultiLayeredConsensusNodeKey returns the key for the layered consensus node. 【consensusParticipant tag = cpt】
func GetMultiLayeredConsensusNodeKey(mlcTag, cpt string) string {
	return buildKey(protocol.MLCNMark, mlcTag, cpt)
}

// GetMultiLayeredConsensusRuleTypeKey returns the key for the layered consensus rule type.
func GetMultiLayeredConsensusRuleTypeKey(mlcTag, cpt string) string {
	return buildKey(protocol.MKCRTMark, mlcTag, cpt)
}

// GetMultiLayeredConsensusRuleContentKey returns the key for the layered consensus rule content.
func GetMultiLayeredConsensusRuleContentKey(mlcTag, cpt, rule string) string {
	return buildKey(protocol.MLCRCMark, mlcTag, cpt, rule)
}

// GetMultiLayeredConsensusParticipantSetKey returns the key for the layered consensus participant set.
// It sorts the cptList to ensure consistent key generation regardless of the input order.
func GetMultiLayeredConsensusParticipantSetKey(mlcTag string, cptList []string) string {
	// Sort the cptList to ensure the key is generated consistently.
	sort.Strings(cptList)

	// Join the sorted cptList with the mlcTag to form the key.
	return buildKey(append([]string{mlcTag}, cptList...)...)
}
