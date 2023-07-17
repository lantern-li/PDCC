/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package rpcserver subscription
package rpcserver

import (
	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"chainmaker.org/chainmaker-go/module/txfilter/filtercommon"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hokaccha/go-prettyjson"
	"github.com/panjf2000/ants/v2"
	"sync"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker-go/module/rpcserver/helper"
	"chainmaker.org/chainmaker-go/module/rpcserver/result"
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
	helper0, err := helper.NewHelper(tx, store, role, s.log, s.subscribeFilterPool)
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
	var (
		sendPool *ants.Pool
		wg       *sync.WaitGroup
	)

	if localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.SendPool.Enable {
		wg = &sync.WaitGroup{}
		sendPool, err = ants.NewPool(localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.SendPool.Size)
		if err != nil {
			return s.errorResultByMessage(codes.Internal, "init subscribe send pool fail. error: %v", err)
		}
		defer sendPool.Release()
	}
	if err := s.sendBlock(server, helper0, subscribeResult, sendPool, wg); err != nil {
		return err
	}
	wg.Wait()
	return nil
}

// sendBlock send block
func (s ApiService) sendBlock(server apiPb.RpcNode_SubscribeServer, helper0 helper.Helper, subscribeResult result.SubscribeResult, pool *ants.Pool, wg *sync.WaitGroup) (err error) {
	s.log.Infof("send block ")
	startTime := time.Now()
	base := helper0.GetBaseHelper()
	defer func() {
		since := time.Since(startTime)
		s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block rpc unsubscribe, subscriber: %v, error: %v, end: %v, start: %v, costs: %v", string(base.Tx.Sender.Signer.MemberInfo), err, base.End, base.Start, since))
	}()
	if base.Start == -1 && base.End == -1 {
		// send new block
		return s.sendNewBlock(server, helper0, subscribeResult, -1, pool, wg)
	}

	if base.End != -1 && base.End <= int64(base.LastBlockHeight) {
		// send history block
		_, err = s.sendHistoryBlock(server, helper0, subscribeResult, pool, wg)
		if err != nil {
			s.log.Errorf("sendHistoryBlock failed, %s", err)
			return err
		}

		return status.Error(codes.OK, "OK")
	}

	// start != -1
	// send history block
	alreadySendHistoryBlockHeight, err := s.sendHistoryBlock(server, helper0, subscribeResult, pool, wg)
	if err != nil {
		s.log.Errorf("send history block failed, %s", err)
		return err
	}

	s.log.Debugf("after sendHistoryBlock, alreadySendHistoryBlockHeight is %d", alreadySendHistoryBlockHeight)
	// send new block
	err = s.sendNewBlock(server, helper0, subscribeResult, alreadySendHistoryBlockHeight, pool, wg)
	if err != nil {
		return err
	}
	return nil
}

// sendNewBlock - send new block to subscriber
func (s *ApiService) sendNewBlock(server apiPb.RpcNode_SubscribeServer, helper0 helper.Helper, subscribeResult result.SubscribeResult, alreadySendHistoryBlockHeight int64, pool *ants.Pool, wg *sync.WaitGroup) (err error) {
	s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send block new."))
	var (
		base = helper0.GetBaseHelper()
		errC = make(chan error)

		block *commonPb.Block

		nextHeight int64
		lastHeight int64
		blockC     = make(chan model.NewBlockEvent, 1)
		chainId    = base.Tx.Payload.ChainId
	)

	updaterCtx, cancelUpdater := context.WithCancel(context.Background())
	defer cancelUpdater()
	err = s.startSubscribeBlockEvent(updaterCtx, &lastHeight, chainId, blockC)
	if err != nil {
		return
	}
	if alreadySendHistoryBlockHeight != -1 {
		// 如果已推送区块高度不为-1则基于已推送区块高度继续推送
		nextHeight = alreadySendHistoryBlockHeight + 1
	} else {
		nextHeight = atomic.LoadInt64(&lastHeight) + 1
	}
	for {
		select {
		case <-server.Context().Done():
			s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block_new|rpc server context done."))
			return nil
		case <-s.ctx.Done():
			s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block_new|api server context done."))
			return nil
		case err = <-errC:
			s.log.Errorf("send_block_new|api server send failed. error: %v", err)
			return err
		case <-blockC:
			start := time.Now()
			if base.End != -1 && nextHeight > base.End {
				s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block_new [%v] beyond the subscription range. end: %v, start: %v", nextHeight, base.End, base.Start))
				return status.Error(codes.OK, "OK")
			}

			for ; nextHeight <= atomic.LoadInt64(&lastHeight); nextHeight++ {
				block, err = helper0.GetStore().GetBlock(uint64(nextHeight))
				if err != nil {
					return fmt.Errorf("get block failed. error: %s", err)
				}

				if block == nil {
					s.log.DebugDynamic(filtercommon.LoggingFixLengthFunc("[%d] current height not commit block.", nextHeight))
					continue
				}
				updateFilterRules(block.Txs, helper0, s.log)

				res, stat, err := subscribeResult.GetResultByBlockInfo(&commonPb.BlockInfo{Block: block}, helper0.Verify)
				if err != nil {
					s.log.Errorf("send_block_new [%v] get result failed. error: %v, end: %v, start: %v", nextHeight, err, base.End, base.Start)
					return err
				}
				if res == nil {
					s.log.Warnf("send_block_new [%v] res is nil. end: %v, start: %v", nextHeight, base.End, base.Start)
					continue
				}
				sendStart := time.Now()
				if localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.SendPool.Enable {
					wg.Add(1)
					err = pool.Submit(func() {
						defer wg.Done()
						if err := server.Send(res); err != nil {
							select {
							case errC <- s.errorResultByMessage(codes.Internal, "[%v] send block info by new failed, %s", nextHeight, err):
							default:
							}
						}
					})
				} else {
					if err := server.Send(res); err != nil {
						return s.errorResultByMessage(codes.Internal, "[%v] send block info by new failed, %s", nextHeight, err)
					}
				}
				if err != nil {
					return err
				}
				sendElapsed := time.Since(sendStart)
				totalElapsed := time.Since(start)
				s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block_new [%v] %v [%d/%d] subscriber:%v,data:%v,"+
					"costs[total:%v,send:%v,db:%d,filter:%v,marshal:%v] ", nextHeight, result.ResultTypeNames[subscribeResult.GetType()], stat.ResultTxCount, stat.TotalTxCount, string(helper0.GetBaseHelper().Tx.Sender.Signer.MemberInfo), len(res.Data), totalElapsed.Milliseconds(), sendElapsed.Milliseconds(), stat.GetBlockElapsed, stat.FilterElapsed, stat.MarshalElapsed))
			}
		}
	}
}

// updateFilterRules update filter rules
func updateFilterRules(txs []*commonPb.Transaction, helper0 helper.Helper, logger *logger.CMLogger) {
	// The system contract has only one transaction
	for i := range txs {
		var (
			tx      = txs[i]
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
	subscribeResult result.SubscribeResult, pool *ants.Pool, wg *sync.WaitGroup) (int64, error) {

	var (
		start = helper0.GetBaseHelper().Start
		end   = helper0.GetBaseHelper().End
		i     int64
		errC  = make(chan error)
	)

	// Iterate from startBlockHeight to endBlockHeight
	if start > -1 {
		i = start
	}
	for {
		select {
		case <-s.ctx.Done():
			s.log.InfoDynamic(filtercommon.LoggingFixLengthFunc("send_block_history|api server context done."))
			return -1, s.errorResultByMessage(codes.Internal, "chainmaker is restarting, please retry later")
		case err := <-errC:
			return -1, err
		default:
			start := time.Now()

			// The default traffic limit is 1000
			if err := s.getRateLimitToken(); err != nil {
				return -1, s.errorResultByError(codes.Internal, err)
			}
			// The final height is not equal to -1, and the current height is greater than the final height
			// Returns a block within the specified height range
			if end != -1 && i > end {
				return i - 1, nil
			}
			// 如果未查询到返回 (nil,nil,nil)
			res, stat, err := subscribeResult.GetResultByHeight(uint64(i), helper0.Verify)
			if err != nil {
				return -1, s.errorResultByMessage(codes.Internal, "get result fail, error: %v", err)
			}

			if res == nil {
				return i - 1, nil
			}

			if res.Data == nil {
				continue
			}

			sendStart := time.Now()
			if localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.SendPool.Enable {
				wg.Add(1)
				err = pool.Submit(func() {
					defer wg.Done()
					if err := server.Send(res); err != nil {
						errC <- s.errorResultByMessage(codes.Internal, "[%v] send block info by history failed, %s", i, err)
					}
				})
			} else {
				if err := server.Send(res); err != nil {
					return -1, s.errorResultByMessage(codes.Internal, "[%v] send block info by history failed, %s", i, err)
				}
			}
			if err != nil {
				return 0, err
			}
			sendElapsed := time.Since(sendStart)
			allElapsed := time.Since(start)
			s.log.Infof("send_block_history [%v] %v [%d/%d] subscriber:%v,data:%v,"+
				"costs[total:%v,send:%v,db:%d,filter:%v,marshal:%v] ", i, result.ResultTypeNames[subscribeResult.GetType()], stat.ResultTxCount, stat.TotalTxCount, string(helper0.GetBaseHelper().Tx.Sender.Signer.MemberInfo), len(res.Data), allElapsed.Milliseconds(), sendElapsed.Milliseconds(), stat.GetBlockElapsed, stat.FilterElapsed, stat.MarshalElapsed)
			i++
		}
	}
}
