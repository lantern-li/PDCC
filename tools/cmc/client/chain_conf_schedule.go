/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"fmt"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"github.com/spf13/cobra"
)

// updateScheduleConfigCMD update schedule config sub command
// @return *cobra.Command
func updateScheduleConfigCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "update schedule command",
		Long:  "update schedule command",
	}
	cmd.AddCommand(updateScheduleTypeCMD())

	return cmd
}

// updateScheduleTypeCMD update schedule type
// @return *cobra.Command
func updateScheduleTypeCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updateScheduleType",
		Short: "update schedule type",
		Long:  "update schedule type",
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateScheduleType()
		},
	}

	attachFlags(cmd, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath, flagUserTlsCrtFilePath, flagUserTlsKeyFilePath, flagChainId,
		flagSdkConfPath, flagOrgId, flagAdminCrtFilePaths, flagAdminKeyFilePaths, flagAdminOrgIds, flagScheduleType, flagAlgorithmType,
	})

	cmd.MarkFlagRequired(flagScheduleType)
	cmd.MarkFlagRequired(flagAlgorithmType)

	return cmd
}

func updateScheduleType() error {

	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
	if err != nil {
		return err
	}
	defer client.Stop()

	adminKeys, adminCrts, adminOrgs, err := util.MakeAdminInfo(client, adminKeyFilePaths, adminCrtFilePaths, adminOrgIds)
	if err != nil {
		return err
	}

	payload, err := client.CreateChainConfigScheduleUpdatePayload(schedulerType, algorithmType)
	if err != nil {
		return fmt.Errorf("create chain config block update payload failed, %s", err.Error())
	}

	endorsementEntrys, err := util.MakeEndorsement(adminKeys, adminCrts, adminOrgs, client, payload)
	if err != nil {
		return err
	}

	resp, err := client.SendChainConfigUpdateRequest(payload, endorsementEntrys, -1, true)
	if err != nil {
		return fmt.Errorf("send chain config update request failed, %s", err.Error())
	}
	err = util.CheckProposalRequestResp(resp, false)
	if err != nil {
		return fmt.Errorf("check proposal request resp failed, %s", err.Error())
	}
	fmt.Printf("response %+v\n", resp)
	return nil
}
