
长安链·ChainMaker, a blockchain platform for building secure, trustworthy value-exchange networks to power the new global digital economy.

ChainMaker aims to use standardized and modularized components to build a blockchain infrastructure that can be utilized to construct various blockchain systems for a wide-range of applications,  advancing blockchain developing from the “pre-industrial” era to the “industrial” era of automated assembly.

# Supplemental Material
Detailed supporting materials for PDCC's experimental evaluation are available in the `ExperimentalRecord/` folder.

To reproduce the experimental results presented in this study, please refer to the following guidelines.

# Build (Vendor)
This repository vendors its Go dependencies under `vendor/` for reproducible/offline builds.

```bash
make chainmaker-vendor
```

The binary will be generated at `bin/chainmaker`.

# Start Chainmaker
## 1. Build executable binaries
```bash
cd PDCC/
make chainmaker-vendor
cp bin/chainmaker build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/bin/
```
## 2. Modify configuration
```bash
vim build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/config/wx-org.chainmaker.org/chainconfig/bc1.yml
```
When modifying the execution mechanism of ChainMaker, please locate the `scheduler` field in the `bc1.yml` configuration file.

Choose any of the following mechanisms based on your requirements. When modifying, ensure that you only change the `process_type` and `algorithm_type` parameters. Keep all other settings unchanged.
### 2.1 PDCC Mechanism
To run ChainMaker with the PDCC mechanism, use the following configuration:
```yaml
scheduler:
  process_type: 1
  algorithm_type: 3
```
### 2.2 OCC Mechanism (Optimistic Concurrency Control)
To run ChainMaker with the OCC mechanism, use the following configuration:
```yaml
scheduler:
  process_type: 0
  algorithm_type: 0
```
### 2.3 Aria Mechanism
To run ChainMaker with the Aria mechanism, use the following configuration:
```yaml
scheduler:
  process_type: 1
  algorithm_type: 6
```
### 2.4 Serial Mechanism
To run ChainMaker with the Serial mechanism, use the following configuration:
```yaml
scheduler:
  process_type: 1
  algorithm_type: 1
```
## 3. Run Chainmaker
Once you have selected your configuration, you can start the ChainMaker node.
```bash
 cd PDCC/scripts/
./cluster_quick_start.sh normal
```
Check whether the process exists.
```bash
ps -ef|grep chainmaker | grep -v grep
```
Check whether the port is listening.
```bash
netstat -lptn | grep 1230
```
## 4. Send transaction loads to conduct pressure testing.
The program for clients to send transactions is available at:
https://github.com/lantern-li/sdk-go-client

**Note**
During the test, you can monitor the real-time status of ChainMaker by running the following command to track the TPS (Transactions Per Second):
```bash
cd PDCC/
tail -f build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/log/system.log | grep -i tps
```
## 5. Performance Statistics
After the benchmark completes, you can use the analysis tools provided by the authors. Detailed instructions are available in tools/README.md.
## Quick Start
```bash
cd PDCC/tools/
go run analyze_tps.go -type XXX
```
Supported `-type` values:
- `wria`: parse PDCC TPS, output line chart 
- `occ1`: parse OCC1 TPS, output line chart 
- `occ2`: parse OCC2 TPS, output line chart
- `aria`: parse Aria TPS, output line chart
- `serial`: parse Serial TPS, output line chart
- `occ1dag`: parse OCC1 DAG building cost (ms) and output a line chart
- `wriaPieChart`: parse PDCC 9-phase timing + TPS, and output an HTML with a “range-linked pie chart”
- `wriaRoundNum`: parse PDCC blockheight vs roundNum, and output a line chart
- `pdccmetadata`: parse PDCC `phase10(roundCommitStats)` cost, and output a line chart

## 6. Stop Chainmaker and Clean Up
Once the testing is complete, use the following commands to stop ChainMaker and clean up relevant data and log files.
```bash
cd PDCC/scripts
./cluster_quick_stop.sh clean
```
# todo in somewhere wira means pdcc

# License

长安链·ChainMaker is made available under the Apache License, Version 2.0 (Apache-2.0), located in the [LICENSE](./LICENSE) file.

# Security Note

This repository contains private keys/certificates under `config/` for development/testing environments. Do **NOT** use them in production.

# Declaration
Project chainMaker-go is in early phase currently. Participation and contribution are highly encouraged. Issues and feedback are welcome to be submitted to [ISSUES](https://git.chainmaker.org.cn/chainmaker/chainmaker-go/-/issues).

chainmaker-go项目当前为开源初期阶段，我们鼓励开发者踊跃参与和贡献。您可以将相关反馈提交至 [ISSUES](https://git.chainmaker.org.cn/chainmaker/issue/-/issues) 
