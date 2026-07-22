
长安链·ChainMaker, a blockchain platform for building secure, trustworthy value-exchange networks to power the new global digital economy.

ChainMaker aims to use standardized and modularized components to build a blockchain infrastructure that can be utilized to construct various blockchain systems for a wide-range of applications,  advancing blockchain developing from the “pre-industrial” era to the “industrial” era of automated assembly.

# Supplemental Material
Detailed supporting materials for PDCC's experimental evaluation are available in the `ExperimentalRecord/` folder.
The source code of PDCC resides in `module/core/common/scheduler/deterministic/pdcc/` folder

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
cd PDCC/
vim build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/config/wx-org.chainmaker.org/chainconfig/bc1.yml
```
When modifying the execution mechanism of ChainMaker, please locate the `scheduler` field in the `bc1.yml` configuration file.

Choose any of the following protocols based on your requirements. When modifying, ensure that you only change the `process_type` and `algorithm_type` parameters. Keep all other settings unchanged.
### 2.1 PDCC Protocol
To run ChainMaker with the PDCC Protocol, use the following configuration:
```yaml
scheduler:
  process_type: 1
  algorithm_type: 3
```
### 2.2 CM-Exe \& CM-Rep
To run native non-deterministic concurrency control protocols of ChainMaker, use the following configuration:
```yaml
scheduler:
  process_type: 0
  algorithm_type: 0
```
### 2.3 Aria Protocol
To run ChainMaker with the Aria Protocol, use the following configuration:
```yaml
scheduler:
  process_type: 1
  algorithm_type: 6
```
### 2.4 Serial
To run ChainMaker with the serial execution, use the following configuration:
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
## 4. Send transaction workloads to conduct pressure testing.
The program for clients to send transactions is available at:
https://anonymous.4open.science/r/sdk-go-client/README.md

**Note**
During the test, you can monitor the real-time status of ChainMaker by running the following command to track the TPS (Transactions Per Second):
```bash
cd PDCC/
tail -f build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/log/system.log | grep -i tps
```
## 5. Performance Statistics
After the benchmark completes, you can use the analysis tools provided by the authors.
## Quick Start
```bash
cd PDCC/tools/
go run analyze_tps.go -type XXX
```
Supported `-type` values:
- `pdcc`: parse PDCC TPS, output line chart 
- `cm-exe`: parse CM-Exe TPS, output line chart 
- `cm-rep`: parse CM-Rep TPS, output line chart
- `aria`: parse Aria TPS, output line chart
- `serial`: parse Serial TPS, output line chart
- `pdccPieChart`: parse PDCC 9-phase timing + TPS, and output an HTML with a “range-linked pie chart”
- `pdccRoundNum`: parse PDCC blockheight vs roundNum, and output a line chart

After that, please open the generated HTML file in your browser to view the interactive charts.

**Note:** If you wish to conduct the next test case(e.g., a different skew or a different workload), it is recommended to restart the chain before proceeding, as follows:
```bash
cd PDCC/scripts
./cluster_quick_stop.sh clean
./cluster_quick_start.sh normal
## Then you can send the next test case using sdk-go-client.
```

**Note:** This open-source version of ChainMaker does not currently support hot-swapping of concurrency control protocols. If you wish to switch the concurrency control protocol, you must first execute the following command to stop the chain, and then return to **Step 2: Modify configuration**.
```bash
cd PDCC/scripts
./cluster_quick_stop.sh clean
```

## 6. Stop Chainmaker and Clean Up
Once the testing is complete, use the following commands to stop ChainMaker and clean up relevant data and log files.

```bash
cd PDCC/scripts
./cluster_quick_stop.sh clean
```

# License

长安链·ChainMaker is made available under the Apache License, Version 2.0 (Apache-2.0), located in the [LICENSE](./LICENSE) file.

# Security Note

This repository contains private keys/certificates under `config/` for development/testing environments. Do **NOT** use them in production.

# Declaration
Project chainMaker-go is in early phase currently. Participation and contribution are highly encouraged. Issues and feedback are welcome to be submitted to [ISSUES](https://git.chainmaker.org.cn/chainmaker/chainmaker-go/-/issues).

chainmaker-go项目当前为开源初期阶段，我们鼓励开发者踊跃参与和贡献。您可以将相关反馈提交至 [ISSUES](https://git.chainmaker.org.cn/chainmaker/issue/-/issues) 
