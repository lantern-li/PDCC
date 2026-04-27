/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chainmaker_sdk_go

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker/pb-go/v2/api"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/pb-go/v2/txassign"
	"chainmaker.org/chainmaker/sdk-go/v2/utils"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	subscribeBufferCap = 256 // 订阅通道的容量
)

// SubscribeBlock block subscription, returns channel of subscribed blocks
func (cc *ChainClient) SubscribeBlock(ctx context.Context, startBlock, endBlock int64, withRWSet,
	onlyHeader bool) (<-chan interface{}, error) {

	payload := cc.CreateSubscribeBlockPayload(startBlock, endBlock, withRWSet, onlyHeader)

	return cc.Subscribe(ctx, payload)
}

// SubscribeBlockWithRule block subscription, returns channel of subscribed blocks
func (cc *ChainClient) SubscribeBlockWithRule(ctx context.Context, startBlock, endBlock int64, withRWSet,
	onlyHeader bool, contractName, method string, ruleType txassign.RuleType) (<-chan interface{}, error) {

	payload := cc.CreateSubscribeBlockWithRulePayload(startBlock, endBlock, withRWSet, onlyHeader, contractName, method,
		ruleType)

	return cc.Subscribe(ctx, payload)
}

// SubscribeTx tx subscription, returns channel of subscribed txs
func (cc *ChainClient) SubscribeTx(ctx context.Context, startBlock, endBlock int64, contractName string,
	txIds []string) (<-chan interface{}, error) {

	payload := cc.CreateSubscribeTxPayload(startBlock, endBlock, contractName, txIds)

	return cc.Subscribe(ctx, payload)
}

// SubscribeTxWithRule tx subscription with rule, returns channel of subscribed txs
func (cc *ChainClient) SubscribeTxWithRule(ctx context.Context, startBlock, endBlock int64, contractName string,
	txIds []string, method string, ruleType txassign.RuleType) (<-chan interface{}, error) {

	payload := cc.CreateSubscribeTxPayloadWithRule(startBlock, endBlock, contractName, txIds, method, ruleType)

	return cc.Subscribe(ctx, payload)
}

// SubscribeTxByPreAlias tx subscription by alias prefix, returns channel of subscribed txs
func (cc *ChainClient) SubscribeTxByPreAlias(ctx context.Context, startBlock, endBlock int64,
	aliasPrefix string) (<-chan interface{}, error) {

	if aliasPrefix == "" {
		return nil, errors.New("alias prefix is empty")
	}

	payload := cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_TX.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeTx_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeTx_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeTx_PRE_ALIAS.String(),
				Value: []byte(aliasPrefix),
			},
		}, defaultSeq, nil,
	)

	return cc.Subscribe(ctx, payload)
}

// SubscribeTxByPreTxId tx subscription by tx id prefix, returns channel of subscribed txs
func (cc *ChainClient) SubscribeTxByPreTxId(ctx context.Context, startBlock, endBlock int64,
	txIdPrefix string) (<-chan interface{}, error) {

	if txIdPrefix == "" {
		return nil, errors.New("tx id prefix is empty")
	}

	payload := cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_TX.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeTx_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeTx_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeTx_PRE_TX_ID.String(),
				Value: []byte(txIdPrefix),
			},
		}, defaultSeq, nil,
	)

	return cc.Subscribe(ctx, payload)
}

// SubscribeTxByPreOrgId tx subscription by org id prefix, returns channel of subscribed txs
func (cc *ChainClient) SubscribeTxByPreOrgId(ctx context.Context, startBlock, endBlock int64,
	orgIdPrefix string) (<-chan interface{}, error) {

	if orgIdPrefix == "" {
		return nil, errors.New("org id prefix is empty")
	}

	payload := cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_TX.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeTx_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeTx_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeTx_PRE_ORG_ID.String(),
				Value: []byte(orgIdPrefix),
			},
		}, defaultSeq, nil,
	)

	return cc.Subscribe(ctx, payload)
}

// SubscribeContractEvent contract event subscription, returns channel of subscribed contract events
func (cc *ChainClient) SubscribeContractEvent(ctx context.Context, startBlock, endBlock int64,
	contractName, topic string) (<-chan interface{}, error) {
	payload := cc.CreateSubscribeContractEventPayload(startBlock, endBlock, contractName, topic)
	return cc.Subscribe(ctx, payload)
}

// SubscribeContractEventWithRuleType contract event subscription with rule type, returns channel of subscribed contract events
func (cc *ChainClient) SubscribeContractEventWithRuleType(ctx context.Context, startBlock, endBlock int64,
	contractName, topic string, method string, ruleType txassign.RuleType) (<-chan interface{}, error) {

	payload := cc.CreateSubscribeContractEventPayloadWithRuleType(startBlock, endBlock, contractName, topic, method, ruleType)

	return cc.Subscribe(ctx, payload)
}

// Subscribe returns channel of subscribed items
func (cc *ChainClient) Subscribe(ctx context.Context, payload *common.Payload) (<-chan interface{}, error) {

	req, err := cc.GenerateTxRequest(payload, nil)
	if err != nil {
		return nil, err
	}

	client, err := cc.pool.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.rpcNode.Subscribe(ctx, req)
	if err != nil {
		return nil, err
	}

	c := make(chan interface{}, subscribeBufferCap)
	go func() {
		defer close(c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				result, err := resp.Recv()
				if err == io.EOF {
					cc.logger.Debugf("[SDK] Subscriber got EOF and stop recv msg")
					return
				}

				if err != nil {
					cc.logger.Errorf("[SDK] Subscriber receive failed, %s", err)
					if statusErr, ok := status.FromError(err); ok {
						if statusErr.Code() == codes.Unknown &&
							strings.Contains(err.Error(), "malformed header: missing HTTP content-type") {
							cc.logger.Info("[SDK] conn corrupted, close and reinit conn")
							client.conn.Close()
							conn, err := cc.pool.initGRPCConnect(client.nodeAddr, client.useTLS, client.caPaths,
								client.caCerts, client.tlsHostName)
							if err != nil {
								cc.pool.getLogger().Errorf("init grpc connection [%s] failed, %s",
									client.ID, err.Error())
								return
							}

							client.conn = conn
							client.rpcNode = api.NewRpcNodeClient(conn)
							return
						}
					}
					return
				}

				var ret interface{}
				switch payload.Method {
				case syscontract.SubscribeFunction_SUBSCRIBE_BLOCK.String():
					blockInfo := &common.BlockInfo{}
					if err = proto.Unmarshal(result.Data, blockInfo); err == nil {
						ret = blockInfo
						break
					}

					blockHeader := &common.BlockHeader{}
					if err = proto.Unmarshal(result.Data, blockHeader); err == nil {
						ret = blockHeader
						break
					}

					cc.logger.Error("[SDK] Subscriber receive block failed, %s", err)
					return
				case syscontract.SubscribeFunction_SUBSCRIBE_TX.String():
					tx := &common.Transaction{}
					if err = proto.Unmarshal(result.Data, tx); err != nil {
						cc.logger.Error("[SDK] Subscriber receive tx failed, %s", err)
						return
					}
					ret = tx
				case syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String():
					events := &common.ContractEventInfoList{}
					if err = proto.Unmarshal(result.Data, events); err != nil {
						cc.logger.Error("[SDK] Subscriber receive contract event failed, %s", err)
						return
					}
					for _, event := range events.ContractEvents {
						c <- event
					}
					continue
				case syscontract.SubscribeFunction_SUBSCRIBE_BLOCK_WITH_RULE.String():
					blockInfo := &common.BlockInfo{}
					if err = proto.Unmarshal(result.Data, blockInfo); err == nil {
						ret = blockInfo
						break
					}

					blockHeader := &common.BlockHeader{}
					if err = proto.Unmarshal(result.Data, blockHeader); err == nil {
						ret = blockHeader
						break
					}

					cc.logger.Error("[SDK] Subscriber receive block failed, %s", err)
					close(c)
					return

				default:
					ret = result.Data
				}

				c <- ret
			}
		}
	}()

	return c, nil
}

// SubscribeWithTxReq returns channel of subscribed items with tx request
func (cc *ChainClient) SubscribeWithTxReq(ctx context.Context, payload *common.Payload,
	req *common.TxRequest) (<-chan interface{}, error) {
	client, err := cc.pool.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.rpcNode.Subscribe(ctx, req)
	if err != nil {
		return nil, err
	}

	c := make(chan interface{}, subscribeBufferCap)
	go func() {
		defer close(c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				result, err := resp.Recv()
				if err == io.EOF {
					cc.logger.Debugf("[SDK] Subscriber got EOF and stop recv msg")
					return
				}

				if err != nil {
					cc.logger.Errorf("[SDK] Subscriber receive failed, %s", err)
					return
				}

				var ret interface{}
				switch payload.Method {
				case syscontract.SubscribeFunction_SUBSCRIBE_BLOCK.String():
					blockInfo := &common.BlockInfo{}
					if err = proto.Unmarshal(result.Data, blockInfo); err == nil {
						ret = blockInfo
						break
					}

					blockHeader := &common.BlockHeader{}
					if err = proto.Unmarshal(result.Data, blockHeader); err == nil {
						ret = blockHeader
						break
					}

					cc.logger.Error("[SDK] Subscriber receive block failed, %s", err)
					return
				case syscontract.SubscribeFunction_SUBSCRIBE_TX.String():
					tx := &common.Transaction{}
					if err = proto.Unmarshal(result.Data, tx); err != nil {
						cc.logger.Error("[SDK] Subscriber receive tx failed, %s", err)
						return
					}
					ret = tx
				case syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String():
					events := &common.ContractEventInfoList{}
					if err = proto.Unmarshal(result.Data, events); err != nil {
						cc.logger.Error("[SDK] Subscriber receive contract event failed, %s", err)
						return
					}
					for _, event := range events.ContractEvents {
						c <- event
					}
					continue

				default:
					ret = result.Data
				}

				c <- ret
			}
		}
	}()

	return c, nil
}

// CreateSubscribeBlockPayload create subscribe block payload
func (cc *ChainClient) CreateSubscribeBlockPayload(startBlock, endBlock int64,
	withRWSet, onlyHeader bool) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_BLOCK.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeBlock_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeBlock_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeBlock_WITH_RWSET.String(),
				Value: []byte(strconv.FormatBool(withRWSet)),
			},
			{
				Key:   syscontract.SubscribeBlock_ONLY_HEADER.String(),
				Value: []byte(strconv.FormatBool(onlyHeader)),
			},
		}, defaultSeq, nil,
	)
}

// CreateSubscribeBlockWithRulePayload create subscribe block payload
func (cc *ChainClient) CreateSubscribeBlockWithRulePayload(startBlock, endBlock int64,
	withRWSet, onlyHeader bool, contractName, method string, ruleType txassign.RuleType) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_BLOCK_WITH_RULE.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeBlock_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeBlock_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeBlock_WITH_RWSET.String(),
				Value: []byte(strconv.FormatBool(withRWSet)),
			},
			{
				Key:   syscontract.SubscribeBlock_ONLY_HEADER.String(),
				Value: []byte(strconv.FormatBool(onlyHeader)),
			},
			{
				Key:   syscontract.SubscribeBlock_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeBlock_METHOD.String(),
				Value: []byte(method),
			},
			{
				Key:   syscontract.SubscribeBlock_RULE_TYPE.String(),
				Value: []byte(strconv.FormatInt(int64(ruleType), 10)),
			},
		}, defaultSeq, nil,
	)
}

// CreateSubscribeTxPayload create subscribe tx payload
func (cc *ChainClient) CreateSubscribeTxPayload(startBlock, endBlock int64,
	contractName string, txIds []string) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_TX.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeTx_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeTx_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeTx_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeTx_TX_IDS.String(),
				Value: []byte(strings.Join(txIds, ",")),
			},
		}, defaultSeq, nil,
	)
}

// CreateSubscribeTxPayloadWithRule create subscribe tx payload with rule
func (cc *ChainClient) CreateSubscribeTxPayloadWithRule(startBlock, endBlock int64,
	contractName string, txIds []string, method string, ruleType txassign.RuleType) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_TX.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeTx_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeTx_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeTx_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeTx_TX_IDS.String(),
				Value: []byte(strings.Join(txIds, ",")),
			},
			{
				Key:   syscontract.SubscribeBlock_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeBlock_METHOD.String(),
				Value: []byte(method),
			},
			{
				Key:   syscontract.SubscribeBlock_RULE_TYPE.String(),
				Value: []byte(strconv.FormatInt(int64(ruleType), 10)),
			},
		}, defaultSeq, nil,
	)
}

// CreateSubscribeContractEventPayload create subscribe contract event payload
func (cc *ChainClient) CreateSubscribeContractEventPayload(startBlock, endBlock int64,
	contractName, topic string) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeContractEvent_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeContractEvent_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeContractEvent_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeContractEvent_TOPIC.String(),
				Value: []byte(topic),
			},
		}, defaultSeq, nil,
	)
}

// CreateSubscribeContractEventPayloadWithRuleType create subscribe contract event payload with rule type
func (cc *ChainClient) CreateSubscribeContractEventPayloadWithRuleType(startBlock, endBlock int64,
	contractName, topic string, method string, ruleType txassign.RuleType) *common.Payload {

	return cc.CreatePayload("", common.TxType_SUBSCRIBE, syscontract.SystemContract_SUBSCRIBE_MANAGE.String(),
		syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String(), []*common.KeyValuePair{
			{
				Key:   syscontract.SubscribeContractEvent_START_BLOCK.String(),
				Value: utils.I64ToBytes(startBlock),
			},
			{
				Key:   syscontract.SubscribeContractEvent_END_BLOCK.String(),
				Value: utils.I64ToBytes(endBlock),
			},
			{
				Key:   syscontract.SubscribeContractEvent_CONTRACT_NAME.String(),
				Value: []byte(contractName),
			},
			{
				Key:   syscontract.SubscribeContractEvent_TOPIC.String(),
				Value: []byte(topic),
			},
			{
				Key:   syscontract.SubscribeBlock_METHOD.String(),
				Value: []byte(method),
			},
			{
				Key:   syscontract.SubscribeBlock_RULE_TYPE.String(),
				Value: []byte(strconv.FormatInt(int64(ruleType), 10)),
			},
		}, defaultSeq, nil,
	)
}
