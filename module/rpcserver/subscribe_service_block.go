/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package rpcserver subscription
package rpcserver

import (
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"encoding/json"
	"fmt"
	"github.com/hokaccha/go-prettyjson"

	"chainmaker.org/chainmaker-go/module/rpcserver/helper"
	"chainmaker.org/chainmaker-go/module/rpcserver/result"
	"chainmaker.org/chainmaker-go/module/subscriber"
	"chainmaker.org/chainmaker-go/module/subscriber/model"
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// dealBlockSubscription - deal block subscribe request
func (s *ApiService) dealBlockSubscription(tx *commonPb.Transaction, server apiPb.RpcNode_SubscribeServer) error {
	s.log.DebugDynamic(func() string {
		output, _ := prettyjson.Marshal(tx)
		return fmt.Sprintf("deal block subscription, tx: %s", string(output))
	})
	onlyHeader, err := helper.GetParameterBool(tx.Payload.Parameters, syscontract.SubscribeBlock_ONLY_HEADER.String())
	if err != nil {
		return s.errorResultByError(codes.InvalidArgument, err)
	}
	parameters, _ := json.Marshal(tx.Payload.Parameters)
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("subscribe befor %v", string(parameters))
	})
	withRWSet, err := helper.GetParameterBool(tx.Payload.Parameters, syscontract.SubscribeBlock_WITH_RWSET.String())
	if err != nil {
		return s.errorResultByError(codes.InvalidArgument, err)
	}

	// get current blockchain store
	store, err := s.chainMakerServer.GetStore(tx.Payload.ChainId)
	if err != nil {
		return s.errorResultByCode(codes.Internal, commonErr.ERR_CODE_GET_STORE, err)
	}

	// get current blockchain
	bc, err := s.chainMakerServer.GetBlockchain(tx.Payload.ChainId)
	if err != nil {
		return s.errorResultByCode(codes.Internal, commonErr.ERR_CODE_GET_BLOCKCHAIN, err)
	}

	// get user role
	role, err := utils.GetRoleFromTx(tx, bc.GetAccessControl())
	if err != nil {
		return s.errorResultByMessage(codes.Internal, "get rule from tx fail, error: %v", err)
	}

	// new helper
	helper0, err := helper.NewHelper(tx, store, role, s.log)
	if err != nil {
		return s.errorResultByError(codes.InvalidArgument, err)
	}

	err = helper0.Validate()
	if err != nil {
		return s.errorResultByMessage(codes.InvalidArgument, "validate parameter fail, error: %v", err)
	}
	rule, err := helper0.FilterRule(false)
	if err != nil {
		return err
	}
	if rule == nil {
		return s.errorResultByMessage(codes.InvalidArgument, "%v rule, filter rule not found, parameters: %v", helper0.GetType().String(), parameters)
	}
	resultType := result.GetResultType(onlyHeader, withRWSet)
	subscribeResult := result.NewSubscribeResult(resultType, store, s.log)
	return s.sendBlock(server, helper0, subscribeResult)
}

// sendBlock send block
func (s ApiService) sendBlock(server apiPb.RpcNode_SubscribeServer, helper0 helper.Helper,
	subscribeResult result.SubscribeResult) (err error) {
	s.log.Infof("send block ")
	baseHelper := helper0.GetBaseHelper()

	if baseHelper.Start == -1 && baseHelper.End == -1 {
		// send new block
		return s.sendNewBlock(server, helper0, subscribeResult, -1)
	}

	if baseHelper.End != -1 && baseHelper.End <= int64(baseHelper.LastBlockHeight) {
		// send history block
		_, err = s.sendHistoryBlock(server, helper0, subscribeResult)
		if err != nil {
			s.log.Errorf("sendHistoryBlock failed, %s", err)
			return err
		}

		return status.Error(codes.OK, "OK")
	}

	// start != -1
	// send history block
	alreadySendHistoryBlockHeight, err := s.sendHistoryBlock(server, helper0, subscribeResult)
	if err != nil {
		s.log.Errorf("send history block failed, %s", err)
		return err
	}

	s.log.Debugf("after sendHistoryBlock, alreadySendHistoryBlockHeight is %d", alreadySendHistoryBlockHeight)
	// send new block
	return s.sendNewBlock(server, helper0, subscribeResult, alreadySendHistoryBlockHeight)
}

// sendNewBlock - send new block to subscriber
func (s *ApiService) sendNewBlock(server apiPb.RpcNode_SubscribeServer, helper0 helper.Helper,
	subscribeResult result.SubscribeResult, alreadySendHistoryBlockHeight int64) (err error) {

	var (
		base            = helper0.GetBaseHelper()
		tx              = base.Tx
		eventSubscriber *subscriber.EventSubscriber
		blockInfo       *commonPb.BlockInfo
	)

	blockCh := make(chan model.NewBlockEvent, 1000)

	chainId := tx.Payload.ChainId
	if eventSubscriber, err = s.chainMakerServer.GetEventSubscribe(chainId); err != nil {
		s.log.Errorf("send_new_block [%v] rpc unsubscribe, error:%v, end: %v, start: %v", err, base.End, base.Start)
		return s.errorResultByCode(codes.Internal, commonErr.ERR_CODE_GET_SUBSCRIBER, err)
	}

	sub := eventSubscriber.SubscribeBlockEvent(blockCh)
	defer func() {
		sub.Unsubscribe()
		s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_new_block [%v] rpc unsubscribe, error: %v, end: %v, start: %v", err, base.End, base.Start))
	}()

	for {
		select {
		case ev := <-blockCh:
			blockInfo = ev.BlockInfo

			if alreadySendHistoryBlockHeight != -1 &&
				int64(blockInfo.Block.Header.BlockHeight) > alreadySendHistoryBlockHeight {
				_, err = s.sendHistoryBlock(server, helper0, subscribeResult)
				if err != nil {
					s.log.Errorf("send_new_block [%v] send history block failed, error: %v, end: %v, start: %v", blockInfo.Block.Header.BlockHeight, err, base.End, base.Start)
					return err
				}

				alreadySendHistoryBlockHeight = -1
				continue
			}

			updateFilterRules(blockInfo, helper0, s.log)

			res, err := subscribeResult.GetResultByBlockInfo(blockInfo, helper0.Verify)
			if err != nil {
				s.log.Errorf("send_new_block [%v] get result failed. error: %v, end: %v, start: %v", blockInfo.Block.Header.BlockHeight, err, base.End, base.Start)
				return err
			}
			if res == nil {
				s.log.Warnf("send_new_block [%v] res is nil. end: %v, start: %v", blockInfo.Block.Header.BlockHeight, base.End, base.Start)
				continue
			}
			if base.End != -1 && int64(blockInfo.Block.Header.BlockHeight) >= base.End {
				s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_new_block [%v] beyond the subscription range. end: %v, start: %v", blockInfo.Block.Header.BlockHeight, base.End, base.Start))
				return status.Error(codes.OK, "OK")
			}

			if err = server.Send(res); err != nil {
				err = fmt.Errorf("send_new_block [%v] send block subscribe result by realtime failed. error:%v, end: %v, start: %v", blockInfo.Block.Header.BlockHeight, err, base.End, base.Start)
				s.log.Error(err)
				return err
			}

			s.log.Infof("send_new_block [%v] send new block success, resultType: %v", blockInfo.Block.Header.BlockHeight, result.ResultTypeNames[subscribeResult.GetType()])
		case <-server.Context().Done():
			s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_new_block|rpc server context done."))
			return nil
		case <-s.ctx.Done():
			s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_new_block|api server context done."))
			return nil
		}
	}
}

// updateFilterRules update filter rules
func updateFilterRules(blockInfo *commonPb.BlockInfo, helper0 helper.Helper, logger *logger.CMLogger) {
	// The system contract has only one transaction
	for i := range blockInfo.Block.Txs {
		var (
			tx      = blockInfo.Block.Txs[i]
			payload = tx.Payload
		)

		// Transaction executed successfully
		if tx.Result.Code != commonPb.TxStatusCode_SUCCESS {
			return
		}

		// The contract name is TX_ASSIGN
		if payload.ContractName != syscontract.SystemContract_TX_ASSIGN.String() {
			return
		}

		// The method is called RegisterRule or UpdateRuleByHeight
		if !(payload.Method == syscontract.TxAssignFunction_RegisterRule.String() ||
			payload.Method == syscontract.TxAssignFunction_UpdateRuleByHeight.String()) {
			return
		}

		_, err := helper0.FilterRule(false)
		if err != nil {
			logger.Errorf("get filter rule fail, error: %v", err)
		}

		logger.Infof("update filter rule success")
	}

}

// sendHistoryBlock - send history block to subscriber
func (s *ApiService) sendHistoryBlock(server apiPb.RpcNode_SubscribeServer, helper0 helper.Helper,
	subscribeResult result.SubscribeResult) (int64, error) {

	var (
		start = helper0.GetBaseHelper().Start
		end   = helper0.GetBaseHelper().End
		i     int64
	)

	// Iterate from startBlockHeight to endBlockHeight
	if start > -1 {
		i = start
	}
	for {
		select {
		case <-s.ctx.Done():
			return -1, s.errorResultByMessage(codes.Internal, "chainmaker is restarting, please retry later")
		default:
			// The default traffic limit is 1000
			if err := s.getRateLimitToken(); err != nil {
				return -1, s.errorResultByError(codes.Internal, err)
			}
			// The final height is not equal to -1, and the current height is greater than the final height
			// Returns a block within the specified height range
			if end != -1 && i > end {
				return i - 1, nil
			}

			res, err := subscribeResult.GetResultByHeight(uint64(i), helper0.Verify)
			if err != nil {
				return -1, s.errorResultByMessage(codes.Internal, "get result fail, error: %v", err)
			}

			if res == nil {
				return i - 1, nil
			}
			i++
			if res.Data == nil {
				continue
			}
			if err := server.Send(res); err != nil {
				return -1, s.errorResultByMessage(codes.Internal, "send block info by history failed, %s", err)
			}
			s.log.Infof("send history block success [%v], resultType: %v", i,
				result.ResultTypeNames[subscribeResult.GetType()])
		}
	}
}
