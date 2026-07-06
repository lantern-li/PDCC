# Tools

This directory contains a few helper scripts/programs. Currently it mainly provides `analyze_tps.go`, which parses TPS/timing data for different schedulers from `system.log` and generates interactive ECharts HTML charts.

## analyze_tps.go

### Features

- Parse performance output in the `ChainMaker` node log `system.log` (TPS, tx count per block, some phase timings, etc.)
- Generate HTML reports (line chart / bar chart / pie chart, etc.) for interactive viewing in a browser

### Prerequisites

- Go 1.18+ (see `tools/go.mod`)
- Parsable logs exist at `build/release/.../log/system.log`

The tool reads the following log path by default (hard-coded in the code):

`../build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/log/system.log`

### Usage

Run in the `tools/` directory:

```bash
go run ./analyze_tps.go -type pdcc
```

Flags:

- `-type`: scheduler/analysis type

Supported `-type` values (kept in sync with the code):

- `pdcc`: parse PDCC TPS, output line chart + bar chart
- `cm-exe`: parse CM-Exe TPS, output line chart + bar chart
- `cm-rep`: parse CM-Rep TPS, output line chart + bar chart
- `aria`: parse Aria TPS, output line chart + bar chart
- `serial`: parse Serial TPS, output line chart + bar chart
- `occ1dag`: parse OCC1 DAG building cost (ms) and output a line chart
- `pdccPieChart`: parse PDCC 9-phase timing + TPS, and output an HTML with a “range-linked pie chart”
- `pdccRoundNum`: parse PDCC blockheight vs roundNum, and output a line chart
- `pdccmetadata`: parse PDCC `phase10(roundCommitStats)` cost, and output a line chart

### Output files

Different `-type` values generate different HTML files (written to the current working directory, usually `tools/`):

- `pdcc/cm-exe/cm-rep/reorder/graph/aria/blockstm/serial`: `tps_performance_analysis.html`
- `occ1dag`: `occ1_dag_building_cost_analysis.html`
- `pdccPieChart`: `piechart.html`
- `pdccRoundNum`: `wria_roundnum.html`
- `pdccmetadata`: `pdcc_metadata_phase10_roundCommitStats.html`

Open the generated HTML in a browser to view the interactive charts.
