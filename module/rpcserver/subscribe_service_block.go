/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package rpcserver

import (
	"chainmaker.org/chainmaker-go/module/rpcserver/helper"
	rpcRes "chainmaker.org/chainmaker-go/module/rpcserver/result"
	"chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"chainmaker.org/chainmaker/localconf/v2"

	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	storePb "chainmaker.org/chainmaker/pb-go/v2/store"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	protocol "chainmaker.org/chainmaker/protocol/v2"
	utils "chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 236，但是sz不走该逻辑
func (s *ApiService) checkDealBlockSubscriptionParams(tx *commonPb.Transaction) (startBlock, endBlock int64,
	withRWSet, onlyHeader bool, err error) {
	for _, kv := range tx.Payload.Parameters {
		if kv.Key == syscontract.SubscribeBlock_START_BLOCK.String() {
			startBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeBlock_END_BLOCK.String() {
			endBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeBlock_WITH_RWSET.String() {
			if string(kv.Value) == TRUE {
				withRWSet = true
			}
		} else if kv.Key == syscontract.SubscribeBlock_ONLY_HEADER.String() {
			if string(kv.Value) == TRUE {
				onlyHeader = true
				withRWSet = false
			}
		}

		if err != nil {
			errCode := commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_BLOCK
			errMsg := s.getErrMsg(errCode, err)
			return 0, 0, false, false,
				status.Error(codes.InvalidArgument, errMsg)
		}
	}

	return startBlock, endBlock, withRWSet, onlyHeader, nil
}

// dealBlockSubscription - deal block subscribe request
func (s *ApiService) dealBlockSubscription(tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer) (retErr error) {
	var (
		err             error
		errMsg          string
		errCode         commonErr.ErrCode
		store           protocol.BlockchainStore
		lastBlockHeight int64
		startBlock      int64
		endBlock        int64
		withRWSet       bool
		onlyHeader      bool
		reqSender       protocol.Role
		txId            = tx.Payload.TxId
		chainId         = tx.Payload.ChainId
		subscribeType   string
		senderAddr      string
	)

	defer func() {
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			// metric subscribe active counter
			s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, "", "").Dec()
			// if the function returns an error, count the number of subscription interruptions
			if retErr != nil {
				s.log.Errorf("dealBlockSubscription encountered an error: %v [txId:%s, sender:%s]", retErr, txId, senderAddr)
				s.metricSubscribeInterruptedCounter.WithLabelValues(chainId, senderAddr, subscribeType, "", "").Inc()
			}
		}
	}()

	subscribeType = syscontract.SubscribeFunction_SUBSCRIBE_BLOCK.String()
	if store, err = s.chainMakerServer.GetStore(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_STORE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s]", txId))
		return status.Error(codes.Internal, errMsg)
	}
	senderAddr, err = s.getTxSenderAddress(store, tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("[txId:%s]", txId))
		return err
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		//metric subscribe total counter
		s.metricSubscribeTotalCounter.WithLabelValues(chainId, senderAddr, subscribeType, "", "").Inc()
		//metric subscribe active counter
		s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, "", "").Inc()
	}

	startBlock, endBlock, withRWSet, onlyHeader, err = s.checkDealBlockSubscriptionParams(tx)
	if err != nil {
		s.log.Warnf(fmt.Sprintf("check deal block subscription params failed, err:%s,[txId:%s].",
			err, txId))
		return err
	}

	if err = s.checkSubscribeBlockHeight(startBlock, endBlock); err != nil {
		errCode = commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_BLOCK
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s]", txId))
		return status.Error(codes.InvalidArgument, errMsg)
	}

	s.log.Infof(
		"Recv block subscribe request: [start:%d]/[end:%d]/[withRWSet:%v]/[onlyHeader:%v]/[txId:%s,chainId:%s]",
		startBlock, endBlock, withRWSet, onlyHeader, txId, chainId)

	// 计算addr之前，统一在日志中返回string的tx.Sender.Signer.MemberInfo
	if lastBlockHeight, err = s.checkAndGetLastBlockHeight(store, startBlock); err != nil {
		if lastBlockHeight > 0 {
			startBlock = lastBlockHeight
			s.log.Warnf("Set startBlock to the latestBlockHeight[txId:%s, sender:%s]", txId, senderAddr)
		} else {
			errCode = commonErr.ERR_CODE_GET_LAST_BLOCK
			errMsg = s.getErrMsg(errCode, err)
			s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, sender:%s]", txId, senderAddr))
			return status.Error(codes.Internal, errMsg)
		}
	}

	reqSender, err = s.getRoleFromTx(tx)
	if err != nil {
		s.log.Warnf("getRoleFromTx failed:%s, [txId:%s, sender:%s].", err, txId, senderAddr)
		return err
	}

	//reqSenderOrgId := tx.Sender.Signer.OrgId

	// new helper
	helper0, err := helper.NewHelper(tx, store, reqSender, s.log, s.subscribeFilterPool)
	if err != nil {
		return s.errorResultByError(codes.InvalidArgument, err)
	}

	err = helper0.Validate()
	if err != nil {
		return s.errorResultByMessage(codes.InvalidArgument, "validate parameter fail, error: %v", err)
	}

	// get filter rule from db
	rule, err := helper0.FilterRule(false) // todo 待优化
	if err != nil {
		return err
	}

	// 当时需求如此，必须注册清分规则后，才允许清分
	if rule == nil {
		parameters, _ := json.Marshal(tx.Payload.Parameters)
		return s.errorResultByMessage(codes.InvalidArgument, "%v rule, filter rule not found, parameters: %v",
			helper0.GetType().String(), parameters)
	}

	resultType := rpcRes.GetResultType(onlyHeader, withRWSet)
	subscribeResult := rpcRes.NewSubscribeResult(resultType, store, s.log)

	return s.sendBlock(tx, server, endBlock, startBlock, senderAddr, helper0, subscribeResult)

	//if startBlock == -1 && endBlock == -1 {
	//	return s.sendNewBlock(store, tx, server, endBlock, withRWSet, onlyHeader,
	//		-1, reqSender, reqSenderOrgId, senderAddr)
	//}
	//
	//if endBlock != -1 && endBlock <= lastBlockHeight {
	//	_, err = s.sendHistoryBlock(store, server, startBlock, endBlock,
	//		withRWSet, onlyHeader, reqSender, reqSenderOrgId, txId, senderAddr)
	//
	//	if err != nil {
	//		s.log.Warnf("sendHistoryBlock failed:%s, [txId:%s, sender:%s].", err, txId, senderAddr)
	//		return err
	//	}
	//
	//	return status.Error(codes.OK, "OK")
	//}
	//
	//alreadySendHistoryBlockHeight, err := s.sendHistoryBlock(store, server, startBlock, endBlock,
	//	withRWSet, onlyHeader, reqSender, reqSenderOrgId, txId, senderAddr)
	//
	//if err != nil {
	//	s.log.Warnf("sendHistoryBlock failed:%s", err)
	//	return err
	//}
	//
	//s.log.Infof("after sendHistoryBlock, alreadySendHistoryBlockHeight is %d, [txId:%s, sender:%s].",
	//	alreadySendHistoryBlockHeight, txId, senderAddr)
	//
	//return s.sendNewBlock(store, tx, server, endBlock, withRWSet, onlyHeader, alreadySendHistoryBlockHeight,
	//	reqSender, reqSenderOrgId, senderAddr)
}

func (s *ApiService) sendBlock(tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer, endBlockHeight int64, startBlock int64,
	senderAddr string, helper0 helper.Helper, subscribeResult rpcRes.SubscribeResult) error {

	var (
		txId = tx.Payload.TxId
	)

	base := helper0.GetBaseHelper()
	if base.Start == -1 && base.End == -1 {
		// send new block
		return s.sendNewBlock(tx, server, endBlockHeight, -1,
			senderAddr, helper0, subscribeResult)
	}

	if base.End != -1 && base.End <= int64(base.LastBlockHeight) {
		// send history block
		_, err := s.sendHistoryBlock(server, startBlock, endBlockHeight, txId, senderAddr, helper0, subscribeResult)
		if err != nil {
			s.log.Warnf("sendHistoryBlock failed:%s, [txId:%s, sender:%s].", err, txId, senderAddr)
			return err
		}

		return status.Error(codes.OK, "OK")
	}

	alreadySendHistoryBlockHeight, err := s.sendHistoryBlock(server, startBlock, endBlockHeight,
		txId, senderAddr, helper0, subscribeResult)

	if err != nil {
		s.log.Warnf("sendHistoryBlock failed:%s", err)
		return err
	}

	s.log.Infof("after sendHistoryBlock, alreadySendHistoryBlockHeight is %d, [txId:%s, sender:%s].",
		alreadySendHistoryBlockHeight, tx.Payload.TxId, senderAddr)

	return s.sendNewBlock(tx, server, endBlockHeight, alreadySendHistoryBlockHeight,
		senderAddr, helper0, subscribeResult)
}

// sendNewBlock - send new block to subscriber
func (s *ApiService) sendNewBlock(tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer,
	endBlockHeight int64, alreadySendHistoryBlockHeight int64,
	senderAddress string, helper0 helper.Helper, subscribeResult rpcRes.SubscribeResult) error {

	var (
		errCode         commonErr.ErrCode
		err             error
		errMsg          string
		lastBlockHeight int64
		chainId         = tx.Payload.ChainId
		txId            = tx.Payload.TxId
		blockC          = make(chan model.NewBlockEvent, 1)
		base            = helper0.GetBaseHelper()
	)

	updaterCtx, cancelUpdater := context.WithCancel(context.Background())
	defer cancelUpdater()
	err = s.startSubscribeBlockEvent(updaterCtx, &lastBlockHeight, chainId, blockC)
	if err != nil {
		errCode = commonErr.ERR_CODE_GET_SUBSCRIBER
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, sender:%s]", txId, senderAddress))
		return status.Error(codes.Internal, errMsg)
	}

	if alreadySendHistoryBlockHeight == -1 {
		alreadySendHistoryBlockHeight = atomic.LoadInt64(&lastBlockHeight)
	}

	for {
		select {
		case <-blockC:
			// 首先判断是否结束发送数据。
			// 注意：当且仅当 endBlockHeight != -1 时，才有可能结束发送数据。
			// 当 endBlockHeight == -1 时，永不结束。
			if base.End != -1 && alreadySendHistoryBlockHeight >= base.End {
				s.log.Infof("endBlockHeight reached[alreadySendHistoryBlockHeight:%d, "+
					"endBlockHeight:%d], [txId:%s, sender:%s].",
					alreadySendHistoryBlockHeight, endBlockHeight, txId, senderAddress)
				return status.Error(codes.OK, "OK")
			}

			if alreadySendHistoryBlockHeight < atomic.LoadInt64(&lastBlockHeight) {
				alreadySendHistoryBlockHeight, err = s.sendHistoryBlock(server, alreadySendHistoryBlockHeight+1,
					endBlockHeight, txId, senderAddress, helper0, subscribeResult)
				if err != nil {
					s.log.Warnf("send history block failed:%s[txId:%s, sender:%s].", err, txId, senderAddress)
					return err
				}
			}
		case <-server.Context().Done():
			s.log.Infof("client server context done[txId:%s, sender:%s].", txId, senderAddress)
			return nil
		case <-s.ctx.Done():
			s.log.Warnf("chain server context done[txId:%s, sender:%s].", txId, senderAddress)
			return nil
		}
	}
}

func (s *ApiService) getTxSenderAddress(store protocol.BlockchainStore, tx *commonPb.Transaction) (string, error) {
	// compatible sz alias
	if tx.Sender.GetSigner().MemberType == accesscontrol.MemberType_ALIAS {
		return string(tx.Sender.GetSigner().MemberInfo), nil
	}

	bcChain, err := s.chainMakerServer.GetBlockchain(tx.Payload.ChainId)
	if err != nil {
		return "", err
	}

	ac := bcChain.GetAccessControl()
	publicKeyPEM, err := publicKeyPEMFromMember(tx.Sender.GetSigner(), store)
	if err != nil {
		return "", err
	}

	addr, _, err := ac.GetAddressFromCache(publicKeyPEM)
	if err != nil {
		return "", err
	}

	return addr, nil
}

// sendHistoryBlock - send history block to subscriber
func (s *ApiService) sendHistoryBlock(server apiPb.RpcNode_SubscribeServer,
	startBlockHeight, endBlockHeight int64, txId, senderAddress string, helper0 helper.Helper,
	subscribeResult rpcRes.SubscribeResult) (int64, error) {

	var (
		err    error
		errMsg string
		res    *commonPb.SubscribeResult
		stat   *rpcRes.Stat
	)

	i := startBlockHeight
	for {
		select {
		case <-server.Context().Done():
			s.log.Infof("client server context done[txId:%s, sender:%s].", txId, senderAddress)
			return -1, nil
		case <-s.ctx.Done():
			s.log.Warnf("chain server context done[txId:%s, sender:%s].", txId, senderAddress)
			return -1, status.Error(codes.Internal, "chainmaker is restarting, please retry later")
		default:
			getTokenStick := utils.CurrentTimeMillisSeconds()
			if err = s.getRateLimitToken(senderAddress); err != nil {
				s.log.Warnf("get rate limit token failed:%s, [txId:%s, sender:%s].", err, txId, senderAddress)
				return -1, status.Error(codes.Internal, err.Error())
			}
			getTokenCost := utils.CurrentTimeMillisSeconds() - getTokenStick

			if endBlockHeight != -1 && i > endBlockHeight {
				return i - 1, nil
			}

			// 如果未查询到返回 (nil,nil,nil)
			res, stat, err = subscribeResult.GetResultByHeight(uint64(i), helper0.FiltTxs)
			if err != nil {
				return -1, s.errorResultByMessage(codes.Internal, "get result fail, error: %v", err)
			}

			// 查询到最新区块时，从此处返回
			if res == nil {
				return i - 1, nil
			}

			sendStartStick := utils.CurrentTimeMillisSeconds()
			if err = server.Send(res); err != nil {
				errMsg = fmt.Sprintf("send block info by history failed:%s", err)
				s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, sender:%s]", txId, senderAddress))
				return -1, status.Error(codes.Internal, errMsg)
			}
			sendCost := utils.CurrentTimeMillisSeconds() - sendStartStick
			totalCost := utils.CurrentTimeMillisSeconds() - getTokenStick

			s.log.Infof("send block info by history[height:%d], [txId:%s, sender:%s, subscriber:%v,data:%d]"+
				"costs[getTokenCost:%d,db:%d,filter:%d,marshal:%d, sendCost:%d, total:%d].",
				i, txId, senderAddress, string(helper0.GetBaseHelper().Tx.Sender.Signer.MemberInfo),
				len(res.Data), getTokenCost, stat.GetBlockElapsed,
				stat.FilterElapsed, stat.MarshalElapsed, sendCost, totalCost)
			i++
		}
	}
}

func (s *ApiService) getBlockSubscribeResult(blockInfo *commonPb.BlockInfo,
	onlyHeader bool) (*commonPb.SubscribeResult, error) {

	var (
		resultBytes []byte
		err         error
	)

	if onlyHeader {
		resultBytes, err = proto.Marshal(blockInfo.Block.Header)
	} else {
		resultBytes, err = proto.Marshal(blockInfo)
	}

	if err != nil {
		errMsg := fmt.Sprintf("marshal block subscribe result failed, %s", err)
		s.log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	result := &commonPb.SubscribeResult{
		Data: resultBytes,
	}

	return result, nil
}

func (s *ApiService) getBlockInfoFromStore(store protocol.BlockchainStore, curblockHeight int64, withRWSet bool,
	reqSender protocol.Role, reqSenderOrgId string) (blockInfo *commonPb.BlockInfo,
	alreadySendHistoryBlockHeight int64, err error) {

	var (
		errMsg         string
		block          *commonPb.Block
		blockWithRWSet *storePb.BlockWithRWSet
	)

	if withRWSet {
		blockWithRWSet, err = store.GetBlockWithRWSets(uint64(curblockHeight))
	} else {
		block, err = store.GetBlock(uint64(curblockHeight))
	}

	if err != nil {
		if withRWSet {
			errMsg = fmt.Sprintf("get block with rwset failed, at [height:%d], %s", curblockHeight, err)
		} else {
			errMsg = fmt.Sprintf("get block failed, at [height:%d], %s", curblockHeight, err)
		}
		s.log.Error(errMsg)
		return nil, -1, errors.New(errMsg)
	}

	if withRWSet {
		if blockWithRWSet == nil {
			return nil, curblockHeight - 1, nil
		}

		blockInfo = &commonPb.BlockInfo{
			Block:     blockWithRWSet.Block,
			RwsetList: blockWithRWSet.TxRWSets,
		}

		// filter txs so that only related ones get passed
		if reqSender == protocol.RoleLight {
			newBlock := utils.FilterBlockTxs(reqSenderOrgId, blockWithRWSet.Block)
			blockInfo = &commonPb.BlockInfo{
				Block:     newBlock,
				RwsetList: blockWithRWSet.TxRWSets,
			}
		}
	} else {
		if block == nil {
			return nil, curblockHeight - 1, nil
		}

		blockInfo = &commonPb.BlockInfo{
			Block:     block,
			RwsetList: nil,
		}

		// filter txs so that only related ones get passed
		if reqSender == protocol.RoleLight {
			newBlock := utils.FilterBlockTxs(reqSenderOrgId, block)
			blockInfo = &commonPb.BlockInfo{
				Block:     newBlock,
				RwsetList: nil,
			}
		}
	}

	// 黑名单交易
	blockInfo.Block = utils.FilterBlockBlacklistTxs(blockInfo.Block)
	blockInfo.RwsetList = utils.FilterBlockBlacklistTxRWSet(blockInfo.RwsetList, blockInfo.Block.Header.ChainId)
	//printAllTxsOfBlock(blockInfo, reqSender, reqSenderOrgId)

	return blockInfo, -1, nil
}
