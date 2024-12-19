package client

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	sdkutils "chainmaker.org/chainmaker/sdk-go/v2/utils"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
)

// systemContractMultiSignCMD system contract multi sign command
// @return *cobra.Command
func systemTestTxCMD() *cobra.Command {
	systemContractMultiSignCmd := &cobra.Command{
		Use:   "test-tx",
		Short: "system contract test tx command",
		Long:  "system contract test tx command",
	}

	systemContractMultiSignCmd.AddCommand(systemCreateTxCMD())
	systemContractMultiSignCmd.AddCommand(systemInvokeTxCMD())

	return systemContractMultiSignCmd
}

// systemContractMultiSignCMD system contract multi sign command
// @return *cobra.Command
func systemCreateTxCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-tx",
		Short: "create tx ",
		Long:  "create tx",
		RunE: func(_ *cobra.Command, _ []string) error {
			return createTx()
		},
	}

	attachFlags(cmd, []string{
		flagAddress, flagAmount,
		flagSdkConfPath,
		flagOrgId, flagChainId,
		flagUserTlsCrtFilePath, flagUserTlsKeyFilePath, flagUserSignCrtFilePath, flagUserSignKeyFilePath,
	})

	cmd.MarkFlagRequired(flagAddress)
	cmd.MarkFlagRequired(flagAmount)

	return cmd
}

// systemContractMultiSignCMD system contract multi sign command
// @return *cobra.Command
func systemInvokeTxCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoke-tx",
		Short: "invoke tx ",
		Long:  "invoke tx",
		RunE: func(_ *cobra.Command, _ []string) error {
			return invokeTx()
		},
	}

	attachFlags(cmd, []string{
		flagTxRequest,
		flagSdkConfPath,
		flagOrgId, flagChainId,
		flagUserTlsCrtFilePath, flagUserTlsKeyFilePath, flagUserSignCrtFilePath, flagUserSignKeyFilePath, flagSyncResult,
	})

	return cmd
}

func createTx() error {
	var (
		err error
	)

	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, false)
	if err != nil {
		return err
	}
	defer client.Stop()
	pairs := make(map[string]string)
	if params != "" {
		err := json.Unmarshal([]byte(params), &pairs)
		if err != nil {
			return err
		}
	}
	txId = sdkutils.GetTimestampTxId()
	//resp, err := transfer(client, address, amount, txId, DEFAULT_TIMEOUT, false)
	//if err != nil {
	//	return fmt.Errorf("transfer failed, %s", err.Error())
	//}

	params := map[string]interface{}{
		"to":    address,
		"value": amount,
	}

	payload := client.CreatePayload(
		txId,
		common.TxType_INVOKE_CONTRACT,
		syscontract.SystemContract_DPOS_ERC20.String(),
		syscontract.DPoSERC20Function_TRANSFER.String(),
		util.ConvertParameters(params),
		0,
		nil)

	txReq, err := client.GenerateTxRequest(payload, nil)
	if err != nil {
		panic(err)
	}

	by, err := proto.Marshal(txReq)
	if err != nil {
		panic(err)
	}

	s := hex.EncodeToString(by)
	fmt.Printf(s)

	return nil

}

func invokeTx() error {
	var (
		err error
	)

	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, false)
	if err != nil {
		return err
	}
	defer client.Stop()
	pairs := make(map[string]string)
	if params != "" {
		err := json.Unmarshal([]byte(params), &pairs)
		if err != nil {
			return err
		}
	}

	txRequest2, err := hex.DecodeString(txRequest)
	if err != nil {
		panic(err)
	}

	tq := new(common.TxRequest)
	err = proto.Unmarshal(txRequest2, tq)
	if err != nil {
		panic(err)
	}

	txsp, err := client.SendTxRequest(tq, DEFAULT_TIMEOUT, syncResult)
	if err != nil {
		panic(err)
	}

	fmt.Printf("txsp: %+v\n", txsp)

	return nil

}
