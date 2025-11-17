/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package serial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic"
	schedulerUtils "chainmaker.org/chainmaker-go/module/core/common/scheduler/utils"
	"chainmaker.org/chainmaker/common/v2/monitor"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	ScheduleTimeout        = 10
	ScheduleWithDagTimeout = 20
)

// SerialScheduler serial scheduler
type SerialScheduler struct {
	lock            sync.Mutex
	exitC           chan bool
	log             protocol.Logger
	chainConf       protocol.ChainConf // chain config
	signer          protocol.SigningMember
	metricVMRunTime *prometheus.HistogramVec
	vmHelper        *deterministic.CommonVMHelper // Shared VM execution helper
}

// NewSerialScheduler building a serial transaction scheduler
func NewSerialScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf, ac protocol.AccessControlProvider) *SerialScheduler {

	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.DebugDynamic(func() string {
		return "use the deterministic serial scheduler"
	})
	var scheduler = &SerialScheduler{
		lock:      sync.Mutex{},
		exitC:     make(chan bool),
		log:       log,
		chainConf: chainConf,
	}

	var err error
	if chainConf.ChainConfig().Core.EnableOptimizeChargeGas {
		scheduler.signer, err = schedulerUtils.InitSigner(chainConf.ChainConfig(), localconf.ChainMakerConfig, log)
		if err != nil {
			log.Fatalf("init signer of scheduler failed: err = %v", err)
			return nil
		}
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		scheduler.metricVMRunTime = monitor.NewHistogramVec(monitor.SUBSYSTEM_CORE_PROPOSER_SCHEDULER, "metric_vm_run_time",
			"VM run time metric", []float64{0.005, 0.01, 0.015, 0.05, 0.1, 1, 10}, "chainId")
	}

	// Initialize common VM helper
	scheduler.vmHelper = deterministic.NewCommonVMHelper(log, chainConf, vmMgr, ac)

	return scheduler
}

func (ts *SerialScheduler) Schedule(block *commonPb.Block, txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot) (map[string]*commonPb.TxRWSet, map[string][]*commonPb.ContractEvent, error) {
	txRwSet, contractEvents, _, err := ts.schedule(block, txBatch, snapshot, protocol.Schedule)
	if err != nil {
		return nil, nil, err
	}
	return txRwSet, contractEvents, nil
}

// SimulateWithDag based on the dag in the block, perform scheduling and execution transactions
func (ts *SerialScheduler) SimulateWithDag(block *commonPb.Block, snapshot protocol.Snapshot) (
	map[string]*commonPb.TxRWSet, map[string]*commonPb.Result, error) {
	txRwSet, _, txResultMap, err := ts.schedule(block, block.Txs, snapshot, protocol.Simulate)
	if err != nil {
		return nil, nil, err
	}
	return txRwSet, txResultMap, nil
}

func (ts *SerialScheduler) Halt() {
	select {
	case ts.exitC <- true:
		// Successfully sent halt signal
	default:
		// No goroutine waiting, ignore
	}
}

func (ts *SerialScheduler) schedule(block *commonPb.Block, txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot, mode protocol.ScheduleMode) (map[string]*commonPb.TxRWSet,
	map[string][]*commonPb.ContractEvent, map[string]*commonPb.Result, error) {

	ts.lock.Lock()
	defer ts.lock.Unlock()
	defer ts.vmHelper.ReleaseContractCache()

	txBatchSize := len(txBatch)
	ts.log.Infof("schedule tx batch start, block_number = %v, size = %d", block.Header.BlockHeight, txBatchSize)

	enableOptimizeChargeGas := schedulerUtils.IsOptimizeChargeGasEnabled(ts.chainConf)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ScheduleWithDagTimeout*time.Second)
	defer cancel()

	finishC := make(chan bool, 1)
	errC := make(chan error, 1)

	startTime := time.Now()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ts.log.Errorf("panic in handleTxsSerially: %v", r)
				errC <- fmt.Errorf("panic in handleTxsSerially: %v", r)
			}
			finishC <- true
		}()
		err := ts.handleTxsSerially(ctx, block, txBatch, snapshot, mode)
		if err != nil {
			ts.log.Errorf("failed to handle txs serially, error: %s", err)
			errC <- err
		}
	}()

	// Wait for finish or timeout in main thread
	select {
	case <-finishC:
		// Normal completion
		cancel() // Cancel context on normal completion
	case <-ctx.Done():
		// Context cancelled (timeout or explicit cancel)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			//超时打包已经执行的交易，未执行交易放入交易池等待下次打包，这样存在共识不过的风险，
			// 共识节点可能打包的交易不一致，需要讨论。
			ts.log.Warnf("schedule timeout after %d seconds", ScheduleWithDagTimeout)
		} else {
			ts.log.Warnf("schedule cancelled: %v", ctx.Err())
			errC <- ctx.Err()
		}
	case <-ts.exitC:
		// Halted by external call
		ts.log.Warnf("schedule halted externally")
		cancel() // Cancel context when halted externally
		errC <- fmt.Errorf("schedule halted externally")
	}

	snapshot.Seal()
	timeCostA := time.Since(startTime)

	// Check for errors
	select {
	case err := <-errC:
		return nil, nil, nil, err
	default:
		// No error
	}

	// if the block is not empty, append the charging gas tx
	if enableOptimizeChargeGas && snapshot.GetSnapshotSize() > 0 {
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("append charge gas tx to block[%d]", block.Header.BlockHeight)
		})
		//TODO add this back
		//ts.appendChargeGasTx(block, snapshot, senderCollection)
	}

	// update block's txs(delete the tx which schedule time out.)
	block.Txs = snapshot.GetTxTable()

	timeCostB := time.Since(startTime)
	ts.log.Infof("schedule tx batch finished, success %d, txs execution cost %v, "+
		"dag building cost %v, total used %v, tps %v", len(block.Txs), timeCostA,
		timeCostB-timeCostA, timeCostB, float64(len(block.Txs))/(float64(timeCostB)/1e9))

	txRWSetMap := schedulerUtils.GetTxRWSetTable(snapshot, block, ts.log)
	contractEventMap := schedulerUtils.GetContractEventMap(block)
	var txResultMap map[string]*commonPb.Result
	// return txResultMap when sync mode
	if mode == protocol.Simulate {
		txResultMap = snapshot.GetTxResultMap()
	}

	return txRWSetMap, contractEventMap, txResultMap, nil
}

func (ts *SerialScheduler) handleTxsSerially(ctx context.Context, block *commonPb.Block, txBatch []*commonPb.Transaction, snapshot protocol.Snapshot, mode protocol.ScheduleMode) error {
	for _, tx := range txBatch {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			ts.log.Warnf("handleTxsSerially cancelled: %v", ctx.Err())
			return ctx.Err()
		default:
			// Continue processing
		}

		if snapshot.IsSealed() {
			return fmt.Errorf("handleTx(`%v`) snapshot has already sealed", tx.GetPayload().TxId)
		}
		var start time.Time
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			start = time.Now()
		}

		// execute tx, and get
		// 1) the read/write set
		// 2) the result that telling if the invoke success.
		txSimContext, specialTxType, runTxSuccess := ts.vmHelper.ExecuteTx(tx, snapshot, block)
		if mode == protocol.Schedule {
			tx.Result = txSimContext.GetTxResult()
		}
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => executeTx(...) => runTxSuccess = %v", tx.GetPayload().TxId, runTxSuccess)
		})

		applyResult, applySize := snapshot.ApplyTxSimContext(txSimContext, specialTxType,
			runTxSuccess, true)
		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("handleTx(`%v`) => ApplyTxSimContext(...) => snapshot.txTable = %v, applySize = %v",
				tx.GetPayload().TxId, len(snapshot.GetTxTable()), applySize)
		})

		if !applyResult {
			return fmt.Errorf("serial scheduler conflict: tx_id=%s, apply_size=%d", tx.GetPayload().TxId, applySize)
		}

		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			elapsed := time.Since(start)
			ts.metricVMRunTime.WithLabelValues(tx.Payload.ChainId).Observe(elapsed.Seconds())
		}

		ts.log.DebugDynamic(func() string {
			return fmt.Sprintf("apply to snapshot success, tx id:%s, result:%+v, apply count:%d",
				tx.Payload.GetTxId(), txSimContext.GetTxResult(), applySize)
		})
	}
	return nil
}
