/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package proposer

import (
	"chainmaker.org/chainmaker-go/module/core/common"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	batch "chainmaker.org/chainmaker/txpool-batch/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"errors"
	"fmt"
	"time"
)

func getFetchBatch(fetchBatches [][]*commonpb.Transaction) []*commonpb.Transaction {
	fetchBatch := make([]*commonpb.Transaction, 0)
	for _, v := range fetchBatches {
		fetchBatch = append(fetchBatch, v...)
	}

	return fetchBatch
}

// getDuration, get propose duration from config.
// If not access from config, use default value.
func getDuration(chainConf protocol.ChainConf) time.Duration {
	if chainConf == nil || chainConf.ChainConfig() == nil {
		return DEFAULTDURATION * time.Millisecond
	}

	duration := chainConf.ChainConfig().Block.BlockInterval
	if duration <= 0 {
		return DEFAULTDURATION * time.Millisecond
	}
	return time.Duration(duration) * time.Millisecond
}

func getCurrentTimeHour() string {
	// 获取当前时间
	now := time.Now()
	// 获取当前日期 (年、月、日)
	year, month, day := now.Date()
	// 获取当前小时
	hour := now.Hour()
	// 返回格式化后的日期和时间
	return fmt.Sprintf("%d-%d-%d %d:00", year, month, day, hour)
}

/*
 * getLastProposeTimeByBlockFinger, get prorpose block time by block finger, it delayed by some second
 */
func getLastProposeTimeByBlockFinger(blockFinger string) (int64, error) {
	timeValue, ok := common.ProposeRepeatTimerMap.Load(blockFinger)
	if !ok {
		timeNow := utils.CurrentTimeMillisSeconds()
		common.ProposeRepeatTimerMap.Store(blockFinger, timeNow)
		return timeNow, nil
	}

	switch timeNow := timeValue.(type) {
	case int64:
		return timeNow, nil
	default:
		timeNow = utils.CurrentTimeMillisSeconds()
		common.ProposeRepeatTimerMap.Store(blockFinger, timeNow)
		errMsg := "propose repeat time map type is wrong"
		return 0, errors.New(errMsg)
	}
}

func getCutBlock(chainConf protocol.ChainConf, block *commonpb.Block, logger protocol.Logger) *commonpb.Block {
	cutBlock := new(commonpb.Block)
	if common.IfOpenConsensusMessageTurbo(chainConf) ||
		common.TxPoolType == batch.TxPoolType {
		cutBlock = common.GetTurboBlock(block, cutBlock, chainConf, logger)
	} else {
		cutBlock = block
	}

	return cutBlock
}