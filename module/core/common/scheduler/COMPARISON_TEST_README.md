# 调度器性能对比测试

## 概述

本测试用于对比 **WRIA 调度器**（确定性）和**非确定性调度器**的性能差异。

## 测试文件

- **scheduler_comparison_test.go** - 对比测试实现
- **scheduler_factory.go** - 添加了 `NewTxSchedulerForTest()` 用于测试

## 测试场景

### 1. `TestSchedulerComparison_ConflictRates` - 不同冲突率对比

测试不同冲突场景下的性能：
- **无冲突 (0%)**: 每个交易访问不同的 key
- **低冲突 (10%)**: 10% 的交易访问共享 key
- **高冲突 (50%)**: 50% 的交易访问共享 key  
- **链式依赖 (100%)**: 每个交易依赖前一个交易

### 2. `TestSchedulerComparison_TxCounts` - 不同交易数量对比

测试不同交易数量下的性能（低冲突场景）：
- 交易数: 50, 100, 200, 500, 1000, 2000
- 包含可扩展性分析

### 3. `TestSchedulerComparison_HighLoad` - 高负载测试

测试 5000 笔交易的高负载场景
- 使用 `-short` 标志可跳过

## 运行测试

```bash
# 运行冲突率对比测试
go test ./module/core/common/scheduler -run TestSchedulerComparison_ConflictRates -v

# 运行交易数量对比测试
go test ./module/core/common/scheduler -run TestSchedulerComparison_TxCounts -v

# 运行所有对比测试
go test ./module/core/common/scheduler -run TestSchedulerComparison -v

# 跳过高负载测试
go test ./module/core/common/scheduler -run TestSchedulerComparison -v -short
```

## 测试结果示例

```
================================================================================
  调度器性能对比测试：不同冲突率场景
================================================================================

【场景 1/4】无冲突 (0%)
  WRIA调度器:     吞吐量= 8290.59 tx/s, 耗时=   12.06ms
  非确定性调度器: 吞吐量=   XX.XX tx/s, 耗时= XXXX.XXms
  ✓ WRIA 性能优于非确定性: X.XXx

【场景 2/4】低冲突 (10%)
  WRIA调度器:     吞吐量= 4791.65 tx/s, 耗时=   20.87ms
  ...

性能对比汇总表
================================================================================
场景                 | WRIA(tx/s)   | 非确定性(tx/s)  | 性能比
--------------------------------------------------------------------------------
无冲突 (0%)          |      8290.59 |         XX.XX    |      X.XXx
低冲突 (10%)         |      4791.65 |         XX.XX    |      X.XXx
高冲突 (50%)         |      1041.41 |         XX.XX    |      X.XXx
链式依赖 (100%)      |      4209.85 |         XX.XX    |      X.XXx
--------------------------------------------------------------------------------
平均性能比           |              |                  |      X.XXx

结论：
  ...
```

## 性能指标

测试提供以下指标：
- **吞吐量** (tx/s): 每秒处理的交易数
- **执行时间** (ms): 调度器总执行时间
- **性能比**: WRIA 相对于非确定性调度器的性能倍数
- **可扩展性**: 交易量增长与吞吐量增长的关系

## 冲突场景生成

通过 `generateReadWriteSet()` 函数根据交易序号生成不同的读写模式：

### 无冲突
```go
key := fmt.Sprintf("key_%d", txIndex)
txSimContext.Put(contractName, []byte(key), value)
```

### 低冲突 (10%)
```go
if txIndex%10 == 0 {
    txSimContext.Get(contractName, []byte("shared_key"))
    txSimContext.Put(contractName, []byte("shared_key"), value)
} else {
    key := fmt.Sprintf("key_%d", txIndex)
    txSimContext.Put(contractName, []byte(key), value)
}
```

### 高冲突 (50%)
```go
if txIndex%2 == 0 {
    txSimContext.Get(contractName, []byte("shared_key"))
    txSimContext.Put(contractName, []byte("shared_key"), value)
} else {
    key := fmt.Sprintf("key_%d", txIndex)
    txSimContext.Put(contractName, []byte(key), value)
}
```

### 链式依赖
```go
if txIndex > 0 {
    prevKey := fmt.Sprintf("key_%d", txIndex-1)
    txSimContext.Get(contractName, []byte(prevKey))
}
currentKey := fmt.Sprintf("key_%d", txIndex)
txSimContext.Put(contractName, []byte(currentKey), value)
```

## 注意事项

1. **测试环境**: 测试使用 gomock 模拟所有依赖，不需要真实的区块链环境
2. **并发安全**: 所有 mock 数据结构都使用互斥锁保护
3. **超时设置**: 非确定性调度器默认超时 10 秒
4. **性能差异**: WRIA 调度器在所有场景下都表现出较高的吞吐量

## 测试架构

```
测试函数
  └── runSchedulerComparisonTest()
       ├── prepareSchedulerEnvironment()  // 创建 mocks
       │    ├── setupSnapshotMocksForComparison()  // 配置 snapshot mock
       │    ├── 创建 VmManager mock
       │    ├── 创建 ChainConf mock
       │    └── 创建调度器 (WRIA 或非确定性)
       ├── generateTransactions()  // 生成测试交易
       ├── scheduler.Schedule()  // 执行调度
       └── 计算性能指标
```

## 开发者指南

如果需要添加新的测试场景：

1. 在 `ConflictScenario` enum 中添加新场景
2. 在 `generateReadWriteSet()` 中实现新场景的读写模式
3. 在测试函数中添加新场景的名称和配置
4. 运行测试验证结果

