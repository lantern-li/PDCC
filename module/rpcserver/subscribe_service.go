/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rpcserver

import (
	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"chainmaker.org/chainmaker-go/module/rpcserver/rateLimiter"

	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	protocol "chainmaker.org/chainmaker/protocol/v2"
	utils "chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// TRUE true string
	TRUE = "true"
)

// SubscribeWS processing requests for message subscription by websocket
func (s *ApiService) SubscribeWS(rawTxReq *commonPb.RawTxRequest, server apiPb.RpcNode_SubscribeWSServer) error {
	var req commonPb.TxRequest

	err := proto.Unmarshal(rawTxReq.RawTx, &req)
	if err != nil {
		err = fmt.Errorf("unmarshal subscribe websocket raw tx failed, %s", err)
		s.log.Error(err.Error())
		return status.Error(codes.Internal, err.Error())
	}

	return s.Subscribe(&req, server)
}

// Subscribe - deal block/tx/contracEvent subscribe request
func (s *ApiService) Subscribe(req *commonPb.TxRequest, server apiPb.RpcNode_SubscribeServer) error {

	var (
		errCode commonErr.ErrCode
		errMsg  string
	)

	tx := &commonPb.Transaction{
		Payload:   req.Payload,
		Sender:    req.Sender,
		Endorsers: req.Endorsers,
		Result:    nil,
		Payer:     req.Payer,
	}

	errCode, errMsg = s.validate(tx)
	if errCode != commonErr.ERR_CODE_OK {
		return status.Error(codes.Unauthenticated, errMsg)
	}

	switch req.Payload.Method {
	case syscontract.SubscribeFunction_SUBSCRIBE_BLOCK.String():
		return s.dealBlockSubscription(tx, server)
	case syscontract.SubscribeFunction_SUBSCRIBE_TX.String():
		return s.dealTxSubscription(tx, server)
	case syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String():
		return s.dealContractEventSubscription(tx, server)
	case syscontract.SubscribeFunction_SUBSCRIBE_BLOCK_WITH_RULE.String():
		return s.dealBlockSubscription(tx, server)
	}

	return nil
}

func (s *ApiService) checkAndGetLastBlockHeight(store protocol.BlockchainStore,
	payloadStartBlockHeight int64) (int64, error) {

	var (
		err             error
		lastBlock       *commonPb.Block
		lastBlockHeight uint64
	)

	if lastBlock, err = store.GetLastBlock(); err != nil {
		return -1, s.errorResultByCode(codes.Internal, commonErr.ERR_CODE_GET_LAST_BLOCK, err)
	}

	lastBlockHeight = lastBlock.Header.BlockHeight

	if int64(lastBlockHeight) < payloadStartBlockHeight {
		errMsg := fmt.Sprintf("payload start block height:%d > last block height:%d",
			payloadStartBlockHeight, lastBlockHeight)

		s.log.Warn(errMsg)

		//return int64(lastBlock.Header.BlockHeight), status.Error(codes.InvalidArgument, errMsg) 236 是这段代码，但是考虑到税总已投入使用且此处影响不大，继续沿用下面原税总处理逻辑
		return -1, s.errorResultByMessage(codes.InvalidArgument, "payload start block height:%d >  last block "+
			"height:%d", payloadStartBlockHeight, lastBlockHeight)
	}

	return int64(lastBlock.Header.BlockHeight), nil
}

func (s *ApiService) getRateLimitToken(sender string) error {
	// disable subscriber rate limiter
	if s.subscriberRateLimiter == nil {
		return nil
	}

	subscriberRateLimiterType := s.subscriberRateLimiter.GetSubscriberRateLimiterType()
	switch subscriberRateLimiterType {
	case rateLimiter.GlobalRateLimiterType:
		return s.subscriberRateLimiter.Wait(s.ctx, nil)
	case rateLimiter.SenderRateLimiterType:
		return s.subscriberRateLimiter.Wait(s.ctx, &rateLimiter.LimiterOptions{Sender: sender})
	default:
		return fmt.Errorf("invaild Subscriber Rate Limiter Type[%d]", subscriberRateLimiterType)
	}
}

// checkSubscribeBlockHeight - check subscriber payload info
func (s *ApiService) checkSubscribeBlockHeight(startBlockHeight, endBlockHeight int64) error {
	if startBlockHeight < -1 || endBlockHeight < -1 ||
		(endBlockHeight != -1 && startBlockHeight > endBlockHeight) {

		return errors.New("invalid start block height or end block height")
	}

	return nil
}

func (s *ApiService) getRoleFromTx(tx *commonPb.Transaction) (protocol.Role, error) {
	bc, err := s.chainMakerServer.GetBlockchain(tx.Payload.ChainId)
	if err != nil {
		return "", s.errorResultByCode(codes.Internal, commonErr.ERR_CODE_GET_BLOCKCHAIN, err)
	}

	ac := bc.GetAccessControl()
	return utils.GetRoleFromTx(tx, ac)
}

func (s *ApiService) startSubscribeBlockEvent(ctx context.Context, lastBlockHeight *int64, chainId string,
	dataC chan model.NewBlockEvent) error {
	db, err := s.chainMakerServer.GetStore(chainId)
	if err != nil {
		return fmt.Errorf("get block failed. error: %s", err)
	}
	lastBlock, err := db.GetLastBlock()
	if err != nil {
		return fmt.Errorf("get last block failed. error: %s", err)
	}
	atomic.StoreInt64(lastBlockHeight, int64(lastBlock.Header.BlockHeight))

	blockEventC := make(chan model.NewBlockEvent, 1)
	eventSubscriber, err := s.chainMakerServer.GetEventSubscribe(chainId)
	if err != nil {
		return fmt.Errorf("get event subscribe. error: %s", err)
	}

	go func() {
		sub := eventSubscriber.SubscribeBlockEvent(blockEventC)
		defer sub.Unsubscribe()

		for {
			select {
			case ev := <-blockEventC:
				atomic.StoreInt64(lastBlockHeight, int64(ev.BlockInfo.Block.Header.BlockHeight))
				select {
				case dataC <- ev:
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *ApiService) startSubscribeContractEvent(ctx context.Context, lastBlockHeight *int64, chainId string,
	dataC chan model.NewContractEvent) error {
	db, err := s.chainMakerServer.GetStore(chainId)
	if err != nil {
		return err
	}
	lastBlock, err := db.GetLastBlock()
	if err != nil {
		return err
	}
	atomic.StoreInt64(lastBlockHeight, int64(lastBlock.Header.BlockHeight))

	contractEventC := make(chan model.NewContractEvent, 1)
	eventSubscriber, err := s.chainMakerServer.GetEventSubscribe(chainId)
	if err != nil {
		return err
	}

	go func() {
		sub := eventSubscriber.SubscribeContractEvent(contractEventC)
		defer sub.Unsubscribe()

		for {
			select {
			case ev := <-contractEventC:
				atomic.StoreInt64(lastBlockHeight, int64(ev.BlockHeight))
				select {
				case dataC <- ev:
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}
