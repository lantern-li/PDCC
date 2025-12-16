# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ChainMaker-Go is an enterprise-grade blockchain platform built in Go 1.16. It implements a modular, plugin-based architecture supporting multiple consensus algorithms, smart contract VMs, and execution strategies.

Current version: v2.3.8
Main development branch: `develop`

## Build & Development Commands

### Building

```bash
# Build the main chainmaker binary
make chainmaker
# Output: bin/chainmaker

# Build with vendor dependencies
make chainmaker-vendor

# Build the CMC command-line tool
make cmc
# Output: bin/cmc

# Build send-tool for testing
make send-tool
```

### Testing

```bash
# Run unit tests with coverage
make ut
# This runs scripts/ut_cover.sh which tests individual modules

# Run tests for a specific module (from scripts directory or root)
cd scripts
./ut_cover.sh "module/core" 40 10
# Parameters: module_path min_coverage% min_comment_coverage%

# Run integration tests (QTA)
make cert-qta    # Certificate-based auth tests
make docker-qta  # Docker contract tests
make sql-qta     # SQL storage tests
```

### Linting

```bash
make lint
# Runs golangci-lint across all modules
```

### Running

```bash
# Start chainmaker with config
./bin/chainmaker start -c config/chainmaker.yml

# Quick cluster setup (4 orgs, 1 consensus node each)
cd scripts
./prepare.sh 4 1
./build_release.sh
./cluster_quick_start.sh normal
```

### Testing Smart Contracts

Use the `cmc` tool (ChainMaker Client) located in `tools/cmc/`:

```bash
# Deploy a WASMER contract
./cmc client contract user create \
  --contract-name=fact \
  --runtime-type=WASMER \
  --byte-code-path=./testdata/claim-wasm-demo/rust-fact-2.0.0.wasm \
  --version=1.0 \
  --sdk-conf-path=./testdata/sdk_config.yml \
  --admin-key-file-paths=... \
  --admin-crt-file-paths=... \
  --sync-result=true

# Invoke a contract
./cmc client contract user invoke \
  --contract-name=fact \
  --method=save \
  --sdk-conf-path=./testdata/sdk_config.yml \
  --params='{"file_name":"test","file_hash":"abc123"}' \
  --sync-result=true
```

## Architecture Overview

### Core Design Principles

1. **Plugin/Provider Registry Pattern**: All extensible components (VMs, consensus engines, tx pools, schedulers) are registered at startup via provider functions in `main/component_registry.go`

2. **Message-Driven Architecture**: Modules communicate via MessageBus (pub-sub system) for loose coupling

3. **Factory Pattern**: Used extensively for runtime component selection (BlockProposer, BlockVerifier, AccessControl)

4. **Atomic Hot-Swapping**: BlockVerifierFactory uses `atomic.Value` for lock-free reads during runtime config updates

### Module Organization

The codebase is organized into modules under `/module`, each with specific responsibilities:

#### **Blockchain Module** (`module/blockchain/`)
Central orchestrator that manages all chain modules. Responsible for:
- Module initialization and lifecycle management
- Startup sequence coordination (Store → Net → VM → Core → Consensus → TxPool → Sync)
- Managing state for store, consensus, txPool, coreEngine, vmManager, netService, etc.

Key files:
- `blockchain_start.go` - Module startup sequence
- `blockchain_init.go` - Module initialization

#### **Core Engine** (`module/core/`)
Most complex module, responsible for block proposal, verification, and commitment. Has two operational modes:

**SyncMode** (`syncmode/`) - Standard execution:
- Execute-on-propose: transactions executed during block proposal
- Single CoreEngine implementation

**MaxBFTMode** (`maxbftmode/`) - For MaxBFT consensus:
- Deferred execution for deterministic schedulers
- Specialized MaxBFT-specific optimizations

Core components:
- **BlockProposer**: Generates new blocks from tx pool using scheduler
- **BlockVerifier**: Validates blocks before consensus commitment (uses Factory pattern)
- **BlockCommitter**: Persists validated blocks to store
- **TxScheduler**: Orchestrates transaction execution in VMs

Key files:
- `core_factory.go` - CoreEngine factory
- `syncmode/core_syncmode_impl.go` - Main CoreEngine implementation
- `syncmode/proposer/block_proposer_factory.go` - Proposer routing
- `syncmode/verifier/block_verifier_factory.go` - Verifier atomic updates

#### **Scheduler/TxScheduler** (`module/core/common/scheduler/`)

Three execution strategies available:

1. **Non-Deterministic (Default)**: `TxScheduler`
   - Random/parallel execution with DAG-based conflict detection
   - Best for throughput
   - Uses conflict window tracking

2. **Serial Scheduler** (`deterministic/serial/`):
   - Sequential execution
   - Deterministic results
   - Used with `ProcessType_EXECUTE_AFTER_PROPOSE`

3. **Reorder Scheduler** (`deterministic/reorder/`):
   - Analyzes conflicts, reorders for parallelism
   - Deterministic and optimized
   - Used with `ProcessType_EXECUTE_AFTER_PROPOSE`

Key files:
- `scheduler.go` - Non-deterministic scheduler
- `scheduler_factory.go` - Scheduler selection logic
- `deterministic/serial/` - Serial scheduler implementation
- `deterministic/reorder/` - Reorder scheduler implementation

**Important**: Signer management was recently refactored - signer is now initialized once at CoreEngine level (not per-scheduler) to prevent memory leaks.

#### **Consensus Module** (`module/consensus/`)

Supports pluggable consensus engines via provider registry:
- **SOLO**: Single node consensus
- **TBFT**: Byzantine Fault Tolerant (primary consensus)
- **RAFT**: Crash-fault tolerant
- **DPOS**: Delegated Proof of Stake (built on TBFT)
- **MAXBFT**: Optimized Byzantine consensus

Consensus engines are external packages registered in `main/component_registry.go`.

Key files:
- `consensus_provider.go` - Consensus registry

#### **Transaction Pool Module** (`module/txpool/`)

Pluggable implementations:
- **SINGLE**: Single transaction per block
- **NORMAL**: Standard tx batching
- **BATCH**: Optimized batch processing

Manages pending transactions, broadcasts proposals, signals core engine when ready.

Key files:
- `tx_pool_provider.go` - TxPool registry

#### **VM Manager** (`module/vm/`)

Supports multiple smart contract VMs via provider registry:
- **GASM**: Gas-aware Wasm
- **WASMER**: Wasmer WebAssembly runtime
- **WXVM**: ChainMaker native
- **EVM**: Ethereum Virtual Machine
- **DOCKERGO**: Docker-based Go execution
- **GO**: Native Go contracts

Each VM provides an `InstancesManager` implementing `protocol.VmInstancesManager`.

Key files:
- `vm_provider.go` - VM registry

#### **Snapshot Module** (`module/snapshot/`)

Read-write set caching layer for uncommitted blocks:
- Manages multiple snapshots (one per candidate block)
- Uses block fingerprints (hash of block without txs) as keys
- Snapshots linked in chain for rollback support
- Auto-cleanup when blocks committed (keeps ~8 snapshots max)
- Essential for parallel proposal exploration

Key files:
- `snapshot_manager.go` - Snapshot lifecycle management

#### **Sync Module** (`module/sync/`)

Block synchronization service:
- **BlockChainSyncServer**: Handles block fetch from peers
- **Scheduler Routine**: Requests blocks from peers
- **Processor Routine**: Validates and commits synced blocks
- Uses block pool and request cache
- Broadcasts sync state on configured interval

Key files:
- `blockchain_sync_server.go` - Block sync orchestration

#### **Network Module** (`module/net/`)

Provider-based (Libp2p or Liquid):
- **NetService**: Abstracts network operations
- Message subscription/broadcast for different message types
- TLS configuration (twoway or disable)
- Used by consensus, sync, core for inter-node communication

Key files:
- `net_service.go` - Network service implementation
- `net_factory.go` - Network provider factory

#### **RPC Server Module** (`module/rpcserver/`)

gRPC-based API service:
- **ApiService**: Implements RpcNodeServer interface
- Supports: invoke, query, subscribe, archive operations
- Rate limiting per subscriber
- Subscription filtering pool
- Metrics tracking for monitoring

Key files:
- `rpc_server.go` - API server initialization
- `api_service.go` - RPC API implementation

#### **Access Control Module** (`module/accesscontrol/`)

Factory-based permission management:
- **CertAC**: Certificate-based access control
- **PermissionedPkAC**: Public-key-based access control
- Validates transaction signatures and permissions
- Chain config subscriber for dynamic updates

### Request/Response Flows

#### Block Proposal Flow
```
TxPool → MessageBus.TxPoolSignal
  → BlockProposer (in CoreEngine)
  → TxScheduler.Schedule() [executes txs, generates rwsets]
  → NewSnapshot (caches rwsets)
  → Propose message to Consensus
```

#### Block Verification Flow
```
Consensus → MessageBus.VerifyBlock/VerifyBlockWithRWSet
  → BlockVerifier.VerifyBlock()
  → TxScheduler.VerifyWithDag() [re-executes, compares rwsets]
  → MessageBus.CommitBlock
```

#### Block Commitment Flow
```
BlockCommitter.AddBlock()
  → PutBlock (store.PutBlock)
  → LedgerCache.SetLastCommittedBlock
  → TxFilter.AddsAndSetHeight [duplicate detection]
  → SnapshotManager.NotifyBlockCommitted [cleanup]
  → PublishContractEvent [to subscribers]
  → MessageBus event notification
```

#### Dynamic Scheduler Switching (on config update)
```
ChainConfig update received
  → CoreEngine.updateChainConfig()
  → Stop old BlockProposer
  → Create new TxScheduler (different algorithm)
  → Create new BlockProposer
  → Update BlockVerifier atomically (via atomic.Value)
  → Start new BlockProposer
```

### Configuration-Driven Behavior

Key chain configuration parameters:
- `Scheduler.ProcessType`: EXECUTE_ON_PROPOSE vs EXECUTE_AFTER_PROPOSE
- `Scheduler.AlgorithmType`: RANDOM, SERIAL, REORDER
- `Contract.EnableSqlSupport`: SQL vs KV store backend
- `Block.TxParameterSize`: Max transaction parameter size
- Consensus type determines auth type compatibility

Runtime updates trigger automatic module reconstruction with zero-downtime switching via atomic.Value for verifiers.

## Important Implementation Notes

### Recent Refactoring

The codebase has undergone several recent improvements:

1. **Signer Management Centralization** (recent commits):
   - Signer initialization moved to CoreEngine to prevent memory leaks
   - Resolves issues from repeated signer creation in schedulers

2. **Scheduler Type Refactoring**:
   - Renamed "scheduler type" to "process type" for clarity
   - Fixed metric memory leaks in scheduler implementations

3. **Reorder Scheduler Unit Tests**:
   - Recent addition of unit tests for reorder scheduler
   - Increased test coverage for deterministic scheduling

### Performance Optimizations

1. **Scheduler-Level**:
   - DAG-based conflict detection (O(n) vs O(n²))
   - Conflict window sliding for reduced memory
   - Parallel execution where no conflicts exist
   - Evidence mode for special contracts

2. **Verifier-Level**:
   - BlockVerifierFactory with atomic.Value for lock-free reads
   - Supports both sync and async verification
   - VerifyBlock and VerifyBlockWithRwSets methods

3. **Snapshot-Level**:
   - Fingerprint-based deduplication
   - Pre-linked snapshot chain for rollback
   - Automatic GC of old snapshots
   - Prevents unbounded memory growth

### Module Dependency Order

**Base Modules** (must init in order):
1. Subscriber (event system)
2. Store (blockchain DB)
3. Cache (LedgerCache)
4. ChainConf (chain configuration)
5. TxFilter (tx duplicate detection)
6. KMS (key management services)

**Extended Modules** (consensus-dependent):
- AccessControl → VM → TxPool → Core → Sync → Consensus

**Startup Order** (strictly sequential):
1. Store → 2. NetService → 3. VM → 4. Core → 5. Consensus → 6. TxPool → 7. Sync

## Working with Schedulers

When modifying scheduler code:

1. **Understand the Execution Strategy**:
   - Execute-on-Propose: Non-deterministic, DAG-based (default)
   - Execute-after-Propose: Deterministic (serial or reorder)

2. **Scheduler Selection Logic** (`scheduler_factory.go`):
   ```go
   if scheduler == nil {
       // Use non-deterministic
       return newTxScheduler(...)
   }

   switch scheduler.ProcessType {
   case EXECUTE_ON_PROPOSE:
       return newTxScheduler(...) // non-deterministic
   case EXECUTE_AFTER_PROPOSE:
       if AlgorithmType == SERIAL:
           return serial.NewSerialScheduler(...)
       else if AlgorithmType == REORDER:
           return reorder.NewReorderTxScheduler(...)
   }
   ```

3. **Signer Management**:
   - Do NOT create signers in individual schedulers
   - Use the signer provided by CoreEngine
   - This prevents memory leaks from repeated signer creation

4. **Testing Schedulers**:
   - Unit tests should cover conflict detection
   - Test both deterministic and non-deterministic paths
   - Verify read-write set generation

## Working with Consensus

When adding or modifying consensus engines:

1. **Registration**: Register in `main/component_registry.go`:
   ```go
   consensus.RegisterConsensusProvider(
       consensusPb.ConsensusType_TBFT,
       func(config *utils.ConsensusImplConfig) (protocol.ConsensusEngine, error) {
           return tbft.New(config)
       },
   )
   ```

2. **Auth Type Compatibility**:
   - DPoS requires specific auth type
   - MAXBFT/RAFT cannot use Public auth
   - Validate compatibility in consensus implementation

3. **MessageBus Integration**:
   - Consensus communicates with Core via MessageBus
   - Topics: ProposeState, VerifyBlock, CommitBlock

## Working with VMs

When adding or modifying VM implementations:

1. **Registration**: Register in `main/component_registry.go`:
   ```go
   vm.RegisterVmProvider(
       "WASMER",
       func(chainId string, configs map[string]interface{},
           kmsProviders map[string]protocol.KMSProvider) (protocol.VmInstancesManager, error) {
           return wasmer.NewInstancesManager(chainId, config, kmsProviders), nil
       })
   ```

2. **Instance Management**:
   - VmManager maintains pool of VM instances
   - Each VM type has its own initialization config
   - Support for native contracts (system contracts)

3. **Contract Execution Flow**:
   ```
   Block → TxScheduler.Schedule()
     → For each transaction:
         → VM.Run(contract_type, method, args)
         → Capture read-write sets
         → Handle gas accounting
         → Track contract events
     → Aggregate rwsets & events
     → Return to core engine
   ```

## Testing Structure

- `test/chain1/` - Certificate-based auth test chain
- `test/chain2/` - SQL storage test chain
- `test/chain3/` - Public key auth test chain
- `test/scenario0_native/` - Native contract tests
- `test/scenario1_evm/` - EVM contract tests
- `test/scenario2_rust/` - Rust/WASM contract tests
- `test/scenario3_dockergo/` - Docker Go contract tests
- `test/scenario4_wasmer_sql/` - WASMER + SQL tests

Each test scenario has Python scripts (chain1.py, chain2.py, chain3.py) that run integration tests.

## Common Pitfalls

1. **Don't modify startup order**: The module initialization sequence in `blockchain_start.go` is critical and order-dependent.

2. **Don't create signers in schedulers**: Use the CoreEngine-provided signer to avoid memory leaks.

3. **Don't bypass the factory pattern**: Use BlockProposerFactory and BlockVerifierFactory for proper scheduler routing.

4. **Don't forget snapshot cleanup**: SnapshotManager auto-cleans, but ensure NotifyBlockCommitted is called.

5. **Don't ignore consensus auth type constraints**: Validate auth type compatibility when configuring consensus.

6. **Don't use blocking operations in MessageBus handlers**: MessageBus is async; handlers should be non-blocking.

## Key Files Reference

### Startup & Initialization
- `main/main.go` - Entry point
- `main/component_registry.go` - Plugin registration
- `module/blockchain/blockchain_start.go` - Module startup sequence
- `module/blockchain/blockchain_init.go` - Module initialization

### Core Engine
- `module/core/core_factory.go` - CoreEngine factory
- `module/core/syncmode/core_syncmode_impl.go` - Main CoreEngine
- `module/core/maxbftmode/` - MaxBFT-specific core engine

### Schedulers
- `module/core/common/scheduler/scheduler.go` - Non-deterministic scheduler
- `module/core/common/scheduler/scheduler_factory.go` - Scheduler selection
- `module/core/common/scheduler/deterministic/serial/` - Serial scheduler
- `module/core/common/scheduler/deterministic/reorder/` - Reorder scheduler

### Block Processing
- `module/core/syncmode/proposer/block_proposer_factory.go` - Proposer routing
- `module/core/syncmode/verifier/block_verifier_factory.go` - Verifier updates
- `module/core/syncmode/committer/block_committer_impl.go` - Block commitment

### Other Modules
- `module/consensus/consensus_provider.go` - Consensus registry
- `module/vm/vm_provider.go` - VM registry
- `module/txpool/tx_pool_provider.go` - TxPool registry
- `module/snapshot/snapshot_manager.go` - Snapshot management
- `module/sync/blockchain_sync_server.go` - Block sync
- `module/rpcserver/api_service.go` - RPC API
- `module/net/net_service.go` - Network service
