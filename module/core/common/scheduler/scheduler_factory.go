/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"fmt"
	"regexp"
	"sync"

	aria "chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/Aria"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/graph"
	"github.com/prometheus/client_golang/prometheus"

	"chainmaker.org/chainmaker/common/v2/monitor"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"

	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/pdcc"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/reorder"
	"chainmaker.org/chainmaker-go/module/core/common/scheduler/deterministic/serial"
	"chainmaker.org/chainmaker-go/module/core/provider/conf"
)

var (
	// 全局的 Prometheus metrics，所有 scheduler 共享
	// 避免每次创建 scheduler 时重复注册导致内存泄漏
	metricContractInvokeCounter *prometheus.CounterVec
	metricOnce                  sync.Once
)

// initSchedulerMetrics 初始化全局 metrics（只执行一次）
func initSchedulerMetrics() {
	metricOnce.Do(func() {
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			metricContractInvokeCounter = monitor.NewCounterVec(
				monitor.SUBSYSTEM_VM,
				monitor.MetricContractInvokeCounter,
				monitor.HelpContractInvokeCounterMetric,
				monitor.ChainId, "contract_name", "runtime_type", "state",
			)
		}
	})
}

type TxSchedulerFactory struct{}

// NewTxScheduler building a transaction scheduler
func (sf TxSchedulerFactory) NewTxScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf,
	storeHelper conf.StoreHelper, ledgerCache protocol.LedgerCache,
	ac protocol.AccessControlProvider, signer protocol.SigningMember,
) protocol.TxScheduler {
	// 初始化全局 metrics（只会执行一次）
	initSchedulerMetrics()
	scheduler := chainConf.ChainConfig().Scheduler
	if scheduler == nil {
		return newTxScheduler(vmMgr, chainConf, storeHelper, ledgerCache, ac, signer, metricContractInvokeCounter)
	}

	if chainConf.ChainConfig().Scheduler != nil && chainConf.ChainConfig().Scheduler.EnableEvidence {
		return newTxSchedulerEvidence(vmMgr, chainConf, storeHelper, ledgerCache)
	}

	switch scheduler.ProcessType {
	case config.ProcessType_EXECUTE_ON_PROPOSE:
		if scheduler.AlgorithmType == config.AlgorithmType_RANDOM {
			return newTxScheduler(vmMgr, chainConf, storeHelper, ledgerCache, ac, signer, metricContractInvokeCounter)
		}
	case config.ProcessType_EXECUTE_AFTER_PROPOSE:
		if scheduler.AlgorithmType == config.AlgorithmType_SERIAL {
			return serial.NewSerialScheduler(vmMgr, chainConf, ac, metricContractInvokeCounter)
		} else if scheduler.AlgorithmType == config.AlgorithmType_REORDER {
			return reorder.NewReorderTxScheduler(vmMgr, chainConf, storeHelper, ac)
		} else if scheduler.AlgorithmType == config.AlgorithmType_WRIA {
			return wria.NewWriaScheduler(vmMgr, chainConf, storeHelper, ac)
		} else if scheduler.AlgorithmType == config.AlgorithmType_GRAPH {
			return graph.NewGraphScheduler(vmMgr, chainConf, storeHelper, ac)
		} else if scheduler.AlgorithmType == config.AlgorithmType_ARIA {
			return aria.NewAriaScheduler(vmMgr, chainConf, storeHelper, ac)
		}
	}
	panic(fmt.Sprintf("invaild scheduler config  %+v", scheduler))
}

// newTxScheduler building a regular transaction scheduler
func newTxScheduler(vmMgr protocol.VmManager, chainConf protocol.ChainConf,
	storeHelper conf.StoreHelper, cache protocol.LedgerCache, ac protocol.AccessControlProvider,
	signer protocol.SigningMember, metricContractInvokeCounter *prometheus.CounterVec,
) *TxScheduler {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Debugf("use the common TxScheduler.")

	txScheduler := &TxScheduler{
		lock:            sync.Mutex{},
		VmManager:       vmMgr,
		scheduleFinishC: make(chan bool),
		log:             log,
		chainConf:       chainConf,
		StoreHelper:     storeHelper,
		ledgerCache:     cache,
		contractCache:   &sync.Map{},
		ac:              ac,
		// 使用全局共享的 metric，避免重复注册导致内存泄漏
		metricContractInvokeCounter: metricContractInvokeCounter,
		signer:                      signer,
	}
	var err error
	txScheduler.keyReg, err = regexp.Compile(protocol.DefaultStateRegex)
	if err != nil {
		log.Fatalf("compile default state regex error %v", err)
	}
	return txScheduler
}

// newTxSchedulerEvidence building an evidence transaction scheduler
func newTxSchedulerEvidence(vmMgr protocol.VmManager, chainConf protocol.ChainConf,
	storeHelper conf.StoreHelper, cache protocol.LedgerCache,
) *TxSchedulerEvidence {
	log := logger.GetLoggerByChain(logger.MODULE_CORE, chainConf.ChainConfig().ChainId)
	log.Debugf("use the evidence TxScheduler.")
	txSchedulerEvidence := &TxSchedulerEvidence{
		delegate: &TxScheduler{
			lock:            sync.Mutex{},
			VmManager:       vmMgr,
			scheduleFinishC: make(chan bool),
			log:             log,
			chainConf:       chainConf,
			StoreHelper:     storeHelper,
			ledgerCache:     cache,
			contractCache:   &sync.Map{},
		},
	}
	var err error
	txSchedulerEvidence.delegate.keyReg, err = regexp.Compile(protocol.DefaultStateRegex)
	if err != nil {
		log.Fatalf("compile default state regex error %v", err)
	}
	//if localconf.ChainMakerConfig.MonitorConfig.Enabled {
	//	txSchedulerEvidence.delegate.metricVMRunTime = monitor.NewHistogramVec(
	//		monitor.SUBSYSTEM_CORE_PROPOSER_SCHEDULER,
	//		"metric_vm_run_time",
	//		"VM run time metric",
	//		[]float64{0.005, 0.01, 0.015, 0.05, 0.1, 1, 2, 5, 10},
	//		"chainId",
	//	)
	//}
	return txSchedulerEvidence
}

// NewTxSchedulerForTest creates a TxScheduler for testing purposes
// 用于测试，可以自定义 metricContractInvokeCounter
func NewTxSchedulerForTest(vmMgr protocol.VmManager, chainConf protocol.ChainConf,
	storeHelper conf.StoreHelper, cache protocol.LedgerCache, ac protocol.AccessControlProvider,
	signer protocol.SigningMember, metricContractInvokeCounter *prometheus.CounterVec,
) *TxScheduler {
	return newTxScheduler(vmMgr, chainConf, storeHelper, cache, ac, signer, metricContractInvokeCounter)
}
