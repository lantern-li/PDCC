package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// TPSData 存储解析出的TPS数据
type TPSData struct {
	BlockHeight int
	TotalTxs    int
	TPS         float64
}

// parseWriaLogFileTps 从日志文件中解析TPS数据（确定性调度器，如WRIA）
func parseWriaLogFileTps(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	// 正则表达式匹配: total time=XXms, total txs=XXX, TPS=XXX.XX, blockheight=XXX
	pattern := regexp.MustCompile(`total time=([\d.]+(?:ms|µs)), total txs=(\d+), TPS=([\d.]+), blockheight=(\d+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 5 {
			totalTxs, _ := strconv.Atoi(matches[2])
			tps, _ := strconv.ParseFloat(matches[3], 64)
			blockHeight, _ := strconv.Atoi(matches[4])

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseOcc1LogFile 从日志文件中解析TPS数据（非确定性调度器）
// 日志格式: schedule tx batch finished, block 7461, success 1000, ... tps 23498.839498061967
func parseOcc1LogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	// 正则表达式匹配: block XXXX, success XXX, ... tps XXXX.XXX
	pattern := regexp.MustCompile(`block (\d+), success (\d+),.*tps ([\d.]+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			blockHeight, _ := strconv.Atoi(matches[1])
			totalTxs, _ := strconv.Atoi(matches[2])
			tps, _ := strconv.ParseFloat(matches[3], 64)

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseOcc2LogFile 从日志文件中解析TPS数据（OCC2调度器）
// 日志格式: simulate with dag finished, block 8, size 1000, time used 41.915583ms, tps 23857.475631437595
func parseOcc2LogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	// 正则表达式匹配: block X, size XXX, ... tps XXXX.XXX
	pattern := regexp.MustCompile(`block (\d+), size (\d+),.*tps ([\d.]+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			blockHeight, _ := strconv.Atoi(matches[1])
			totalTxs, _ := strconv.Atoi(matches[2])
			tps, _ := strconv.ParseFloat(matches[3], 64)

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseBlockSTMLogFile 从日志文件中解析TPS数据（BlockSTM调度器）
// 日志格式: BlockSTM schedule completed, total time=XXms, total txs=XXX, TPS=XXX.XX, blockheight=XXX
func parseBlockSTMLogFile(logPath string) ([]TPSData, error) {
	return parseWriaLogFileTps(logPath)
}

// parseSerialLogFile 从日志文件中解析TPS数据（Serial调度器）
// 日志格式: [Serial] schedule tx batch finished, blockheight XXX, success XXX, ..., tps XXX.XXX
func parseSerialLogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	pattern := regexp.MustCompile(`\[Serial\] schedule tx batch finished, blockheight (\d+), success (\d+),.*tps ([\d.]+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)
		if len(matches) == 4 {
			blockHeight, _ := strconv.Atoi(matches[1])
			totalTxs, _ := strconv.Atoi(matches[2])
			tps, _ := strconv.ParseFloat(matches[3], 64)
			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseAriaLogFile 从日志文件中解析TPS数据（Aria调度器）
// 日志格式: Aria schedule completed after X rounds, total time=XXms, total txs=XXX, TPS=XXX.XX, blockheight=XXX
func parseAriaLogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	pattern := regexp.MustCompile(`Aria schedule completed after \d+ rounds, total time=[\d.]+(?:ms|µs|s), total txs=(\d+), TPS=([\d.]+), blockheight=(\d+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			totalTxs, _ := strconv.Atoi(matches[1])
			tps, _ := strconv.ParseFloat(matches[2], 64)
			blockHeight, _ := strconv.Atoi(matches[3])

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseGraphLogFile 从日志文件中解析TPS数据（Graph调度器）
// 日志格式: Graph schedule completed after X rounds, total time=XXms, total txs=XXX, TPS=XXX.XX, blockheight=XXX
func parseGraphLogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	// 正则表达式匹配: total time=XXms, total txs=XXX, TPS=XXX.XX, blockheight=XXX
	pattern := regexp.MustCompile(`Graph schedule completed after \d+ rounds, total time=[\d.]+(?:ms|µs|s), total txs=(\d+), TPS=([\d.]+), blockheight=(\d+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			totalTxs, _ := strconv.Atoi(matches[1])
			tps, _ := strconv.ParseFloat(matches[2], 64)
			blockHeight, _ := strconv.Atoi(matches[3])

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// parseReorderLogFile 从日志文件中解析TPS数据（Reorder调度器）
// 日志格式: schedule tx batch finished, block 7, success 1000, txs pre-execution cost 23.903252ms, ... tps 19649.08695605187
func parseReorderLogFile(logPath string) ([]TPSData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	// 正则表达式匹配: block X, success XXX, ... tps XXXX.XXX
	pattern := regexp.MustCompile(`block (\d+), success (\d+),.*tps ([\d.]+)`)

	var data []TPSData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			blockHeight, _ := strconv.Atoi(matches[1])
			totalTxs, _ := strconv.Atoi(matches[2])
			tps, _ := strconv.ParseFloat(matches[3], 64)

			data = append(data, TPSData{
				BlockHeight: blockHeight,
				TotalTxs:    totalTxs,
				TPS:         tps,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// RoundNumData 存储区块高度与轮次数据
type RoundNumData struct {
	BlockHeight int
	RoundNum    int
}

// parseWriaRoundNum 从日志文件中解析 WRIA 的 blockheight 和 roundNum
func parseWriaRoundNum(logPath string) ([]RoundNumData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	pattern := regexp.MustCompile(`WRIA schedule completed after (\d+) rounds,.*blockheight=(\d+)`)

	var data []RoundNumData
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		m := pattern.FindStringSubmatch(scanner.Text())
		if len(m) == 3 {
			roundNum, _ := strconv.Atoi(m[1])
			blockHeight, _ := strconv.Atoi(m[2])
			data = append(data, RoundNumData{BlockHeight: blockHeight, RoundNum: roundNum})
		}
	}
	return data, scanner.Err()
}

// createRoundNumLineChart 创建 blockheight vs roundNum 折线图
func createRoundNumLineChart(data []RoundNumData) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "WRIA 调度器: Block Height vs Round Number",
			Subtitle: "区块高度与执行轮次的关系",
		}),
		charts.WithTooltipOpts(opts.Tooltip{}),
		charts.WithLegendOpts(opts.Legend{}),
		charts.WithDataZoomOpts(opts.DataZoom{Type: "slider", Start: 0, End: 100}),
	)

	xAxis := make([]string, len(data))
	items := make([]opts.LineData, len(data))
	for i, d := range data {
		xAxis[i] = fmt.Sprintf("%d", d.BlockHeight)
		items[i] = opts.LineData{Value: d.RoundNum}
	}

	line.SetXAxis(xAxis).
		AddSeries("RoundNum", items).
		SetSeriesOptions(
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{Name: "平均值", Type: "average"}),
		)
	return line
}

// PhaseTimeData 存储9个阶段的耗时数据
type PhaseTimeData struct {
	BlockHeight           int
	Phase1Selection       float64 // ms
	Phase2Execution       float64
	Phase3VersionTagging  float64
	Phase4Merging         float64
	Phase5ConflictDetect  float64
	Phase6Revalidation    float64
	Phase7Commit          float64
	Phase8TxReset         float64
	Phase9BatchSizeAdjust float64
}

// parseDurationMs 将 Go duration 字符串解析为毫秒
func parseDurationMs(s string) float64 {
	// 支持 ns, µs, ms, s
	if len(s) == 0 {
		return 0
	}
	if s[len(s)-2:] == "ms" {
		v, _ := strconv.ParseFloat(s[:len(s)-2], 64)
		return v
	}
	if len(s) >= 3 && s[len(s)-3:] == "µs" {
		v, _ := strconv.ParseFloat(s[:len(s)-3], 64)
		return v / 1000
	}
	if len(s) >= 2 && s[len(s)-2:] == "ns" {
		v, _ := strconv.ParseFloat(s[:len(s)-2], 64)
		return v / 1e6
	}
	if s[len(s)-1:] == "s" {
		v, _ := strconv.ParseFloat(s[:len(s)-1], 64)
		return v * 1000
	}
	return 0
}

// parseWriaPhaseTime 从日志文件中解析9个阶段的耗时数据
func parseWriaPhaseTime(logPath string) ([]PhaseTimeData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	pattern := regexp.MustCompile(
		`phase1\(selection\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase2\(execution\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase3\(versionTagging\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase4\(merging\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase5\(conflictDetection\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase6\(revalidation\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase7\(commit\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase8\(txReset\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase9\(batchSizeAdjust\)=([\d.]+(?:ns|µs|ms|s))`,
	)
	blockHeightPattern := regexp.MustCompile(`blockheight=(\d+)`)

	var data []PhaseTimeData
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		m := pattern.FindStringSubmatch(line)
		if len(m) == 10 {
			blockHeight := 0
			if bm := blockHeightPattern.FindStringSubmatch(line); len(bm) == 2 {
				blockHeight, _ = strconv.Atoi(bm[1])
			}
			data = append(data, PhaseTimeData{
				BlockHeight:           blockHeight,
				Phase1Selection:       parseDurationMs(m[1]),
				Phase2Execution:       parseDurationMs(m[2]),
				Phase3VersionTagging:  parseDurationMs(m[3]),
				Phase4Merging:         parseDurationMs(m[4]),
				Phase5ConflictDetect:  parseDurationMs(m[5]),
				Phase6Revalidation:    parseDurationMs(m[6]),
				Phase7Commit:          parseDurationMs(m[7]),
				Phase8TxReset:         parseDurationMs(m[8]),
				Phase9BatchSizeAdjust: parseDurationMs(m[9]),
			})
		}
	}
	return data, scanner.Err()
}

// createInteractivePieChartHTML 生成带联动的交互式HTML：上方饼图根据下方TPS折线图的DataZoom区间动态更新
func createInteractivePieChartHTML(phaseData []PhaseTimeData, tpsData []TPSData, outputPath string) error {
	// 序列化 phaseData 为 JS 数组
	var phaseBuf strings.Builder
	phaseBuf.WriteString("[")
	for i, d := range phaseData {
		label := fmt.Sprintf("%d", d.BlockHeight)
		if d.BlockHeight == 0 {
			label = fmt.Sprintf("%d", i+1)
		}
		if i > 0 {
			phaseBuf.WriteString(",")
		}
		fmt.Fprintf(&phaseBuf, `{"idx":%d,"label":"%s","p1":%f,"p2":%f,"p3":%f,"p4":%f,"p5":%f,"p6":%f,"p7":%f,"p8":%f,"p9":%f}`,
			i, label,
			d.Phase1Selection, d.Phase2Execution, d.Phase3VersionTagging,
			d.Phase4Merging, d.Phase5ConflictDetect, d.Phase6Revalidation,
			d.Phase7Commit, d.Phase8TxReset, d.Phase9BatchSizeAdjust)
	}
	phaseBuf.WriteString("]")
	phaseJS := phaseBuf.String()

	// 序列化 tpsData 为 JS 数组
	var tpsBuf strings.Builder
	tpsBuf.WriteString("[")
	for i, d := range tpsData {
		if i > 0 {
			tpsBuf.WriteString(",")
		}
		fmt.Fprintf(&tpsBuf, `{"blockHeight":%d,"tps":%f}`, d.BlockHeight, d.TPS)
	}
	tpsBuf.WriteString("]")
	tpsJS := tpsBuf.String()

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>WRIA 阶段耗时分析</title>
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
body { margin: 0; padding: 16px; background: #fff; font-family: sans-serif; }
h2 { text-align: center; margin-bottom: 4px; }
#info { text-align: center; color: #666; margin-bottom: 12px; font-size: 13px; }
#pieChart { width: 100%%; height: 420px; }
#lineChart { width: 100%%; height: 320px; margin-top: 16px; }
</style>
</head>
<body>
<h2>PDCC 各阶段平均耗时分布</h2>
<div id="info">拖动下方折线图的滑块选择区间，饼图将自动更新</div>
<div id="pieChart"></div>
<div id="lineChart"></div>
<script>
var phaseData = %s;
var tpsData = %s;
var phaseNames = ["1.Selection","2.Execution","3.VersionTagging","4.Merging","5.ConflictDetection","6.Revalidation","7.Commit","8.TxReset","9.BatchSizeAdjust"];
var phaseKeys = ["p1","p2","p3","p4","p5","p6","p7","p8","p9"];

var pieChart = echarts.init(document.getElementById('pieChart'));
var lineChart = echarts.init(document.getElementById('lineChart'));

function calcPieData(startIdx, endIdx) {
    var sums = [0,0,0,0,0,0,0,0,0];
    var count = 0;
    for (var i = startIdx; i <= endIdx && i < phaseData.length; i++) {
        for (var j = 0; j < 9; j++) {
            sums[j] += phaseData[i][phaseKeys[j]];
        }
        count++;
    }
    if (count === 0) return [];
    return phaseNames.map(function(name, j) {
        var avg = sums[j] / count;
        return { name: name + '(' + avg.toFixed(3) + 'ms)', value: avg };
    });
}

function updatePie(startPct, endPct) {
    var total = phaseData.length;
    var startIdx = Math.round(startPct / 100 * total);
    var endIdx = Math.round(endPct / 100 * total) - 1;
    if (startIdx < 0) startIdx = 0;
    if (endIdx >= total) endIdx = total - 1;
    if (startIdx > endIdx) return;
    var count = endIdx - startIdx + 1;
    var items = calcPieData(startIdx, endIdx);
    pieChart.setOption({
        title: [{
            text: 'PDCC 各阶段平均耗时分布',
            subtext: '基于区块索引 ' + startIdx + ' ~ ' + endIdx + ' 共 ' + count + ' 个区块',
            left: 'center'
        }],
        series: [{ data: items }]
    });
}

// 初始化饼图
pieChart.setOption({
    tooltip: { formatter: '{b}: {d}%%' },
    legend: { orient: 'vertical', right: '5%%', top: '20%%' },
    series: [{
        type: 'pie',
        radius: '60%%',
        center: ['40%%', '55%%'],
        data: calcPieData(0, phaseData.length - 1),
        label: { formatter: '{b}' }
    }]
});

// 初始化折线图
var tpsXAxis = tpsData.map(function(d) { return d.blockHeight > 0 ? String(d.blockHeight) : ''; });
var tpsValues = tpsData.map(function(d) { return d.tps; });

lineChart.setOption({
    title: [{ text: 'WRIA TPS 折线图', left: 'center' }],
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: tpsXAxis, name: 'Block Height' },
    yAxis: { type: 'value', name: 'TPS' },
    dataZoom: [
        { type: 'slider', start: 0, end: 100, bottom: 10 },
        { type: 'inside', start: 0, end: 100 }
    ],
    series: [{
        name: 'TPS',
        type: 'line',
        data: tpsValues,
        smooth: true,
        markLine: { data: [{ type: 'average', name: '平均值' }] }
    }]
});

// 联动：折线图 DataZoom 变化时更新饼图
lineChart.on('datazoom', function(params) {
    var start = 0, end = 100;
    if (params.batch) {
        start = params.batch[0].start;
        end = params.batch[0].end;
    } else {
        start = params.start !== undefined ? params.start : start;
        end = params.end !== undefined ? params.end : end;
    }
    // TPS 和 phaseData 按索引对齐，用 TPS 的区间比例映射到 phaseData
    var tpsTotal = tpsData.length;
    var phaseTotal = phaseData.length;
    var tpsStart = Math.round(start / 100 * tpsTotal);
    var tpsEnd = Math.round(end / 100 * tpsTotal) - 1;
    // 将 TPS 区间映射到 phaseData 区间（按比例）
    var phaseStart = Math.round(tpsStart / tpsTotal * phaseTotal);
    var phaseEnd = Math.round((tpsEnd + 1) / tpsTotal * phaseTotal) - 1;
    if (phaseStart < 0) phaseStart = 0;
    if (phaseEnd >= phaseTotal) phaseEnd = phaseTotal - 1;
    updatePie(start, end);
});

window.addEventListener('resize', function() {
    pieChart.resize();
    lineChart.resize();
});
</script>
</body>
</html>`, phaseJS, tpsJS)

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(html)
	return err
}

// calculateStats 计算统计信息
func calculateStats(data []TPSData) {
	if len(data) == 0 {
		return
	}

	minTPS := data[0].TPS
	maxTPS := data[0].TPS
	sumTPS := 0.0
	minTxs := data[0].TotalTxs
	maxTxs := data[0].TotalTxs

	for _, d := range data {
		if d.TPS < minTPS {
			minTPS = d.TPS
		}
		if d.TPS > maxTPS {
			maxTPS = d.TPS
		}
		sumTPS += d.TPS

		if d.TotalTxs < minTxs {
			minTxs = d.TotalTxs
		}
		if d.TotalTxs > maxTxs {
			maxTxs = d.TotalTxs
		}
	}

	avgTPS := sumTPS / float64(len(data))

	fmt.Println("============================================================")
	fmt.Println("性能统计信息")
	fmt.Println("============================================================")
	fmt.Printf("总块数: %d\n", len(data))
	fmt.Printf("块高度范围: %d - %d\n", data[0].BlockHeight, data[len(data)-1].BlockHeight)
	fmt.Printf("总交易数范围: %d - %d\n", minTxs, maxTxs)
	fmt.Println("\nTPS 统计:")
	fmt.Printf("  最小 TPS: %.2f\n", minTPS)
	fmt.Printf("  最大 TPS: %.2f\n", maxTPS)
	fmt.Printf("  平均 TPS: %.2f\n", avgTPS)
	fmt.Println("============================================================\n")
}

// createLineChart 创建BlockHeight vs TPS折线图
func createLineChart(data []TPSData, schedulerName string) *charts.Line {
	line := charts.NewLine()

	// 设置全局选项
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s调度器性能分析: Block Height vs TPS", schedulerName),
			Subtitle: "区块高度与TPS的关系",
		}),
		charts.WithTooltipOpts(opts.Tooltip{}),
		charts.WithLegendOpts(opts.Legend{}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 0,
			End:   100,
		}),
	)

	// 准备X轴数据（block height）
	xAxis := make([]string, len(data))
	for i, d := range data {
		xAxis[i] = fmt.Sprintf("%d", d.BlockHeight)
	}

	// 准备Y轴数据（TPS）
	tpsItems := make([]opts.LineData, len(data))
	for i, d := range data {
		tpsItems[i] = opts.LineData{Value: d.TPS}
	}

	line.SetXAxis(xAxis).
		AddSeries("TPS", tpsItems).
		SetSeriesOptions(
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
				Name: "平均值",
				Type: "average",
			}),
		)

	return line
}

// createBarChart 创建TPS分布柱状图
func createBarChart(data []TPSData, schedulerName string) *charts.Bar {
	bar := charts.NewBar()

	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s调度器性能分析: TPS趋势", schedulerName),
			Subtitle: "按区块高度显示TPS变化",
		}),
		charts.WithTooltipOpts(opts.Tooltip{}),
		charts.WithLegendOpts(opts.Legend{}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 0,
			End:   100,
		}),
	)

	// 准备X轴数据（block height）
	xAxis := make([]string, len(data))
	for i, d := range data {
		xAxis[i] = fmt.Sprintf("%d", d.BlockHeight)
	}

	// 准备Y轴数据（TPS）
	tpsItems := make([]opts.BarData, len(data))
	for i, d := range data {
		tpsItems[i] = opts.BarData{Value: d.TPS}
	}

	bar.SetXAxis(xAxis).
		AddSeries("TPS", tpsItems)

	return bar
}

func main() {
	// 命令行参数
	schedulerType := flag.String("type", "wria", "调度器类型: wria, occ1, occ2, reorder, graph, aria, blockstm, serial, wriaPieChart 或 wriaRoundNum")
	flag.Parse()

	logFile := filepath.Join("..", "build", "release", "chainmaker-v2.3.8-wx-org.chainmaker.org", "log", "system.log")
	var schedulerName string
	var data []TPSData
	var err error

	// wriaRoundNum 单独处理：绘制区块高度与 roundNum 关系图
	if *schedulerType == "wriaRoundNum" {
		fmt.Printf("正在解析 WRIA roundNum 数据: %s\n", logFile)
		roundData, err2 := parseWriaRoundNum(logFile)
		if err2 != nil {
			fmt.Printf("错误: %v\n", err2)
			os.Exit(1)
		}
		if len(roundData) == 0 {
			fmt.Println("警告: 未找到 roundNum 数据")
			os.Exit(1)
		}
		fmt.Printf("成功解析 %d 条记录\n", len(roundData))
		page := components.NewPage()
		page.AddCharts(createRoundNumLineChart(roundData))
		outputPath := filepath.Join("wria_roundnum.html")
		f, err3 := os.Create(outputPath)
		if err3 != nil {
			fmt.Printf("创建输出文件失败: %v\n", err3)
			os.Exit(1)
		}
		defer f.Close()
		if err3 = page.Render(f); err3 != nil {
			fmt.Printf("渲染图表失败: %v\n", err3)
			os.Exit(1)
		}
		fmt.Printf("图表已保存至: %s\n", outputPath)
		return
	}

	// wriaPieChart 单独处理：绘制阶段耗时饼图 + TPS折线图，支持滑动区间选择
	if *schedulerType == "wriaPieChart" {
		fmt.Printf("正在解析阶段耗时 (WRIA PieChart): %s\n", logFile)
		phaseData, err2 := parseWriaPhaseTime(logFile)
		if err2 != nil {
			fmt.Printf("错误: %v\n", err2)
			os.Exit(1)
		}
		if len(phaseData) == 0 {
			fmt.Println("警告: 未找到阶段耗时数据")
			os.Exit(1)
		}
		fmt.Printf("正在解析 WRIA TPS 数据: %s\n", logFile)
		tpsData, err3 := parseWriaLogFileTps(logFile)
		if err3 != nil {
			fmt.Printf("错误: %v\n", err3)
			os.Exit(1)
		}
		fmt.Printf("成功解析 %d 条阶段耗时记录, %d 条TPS记录\n", len(phaseData), len(tpsData))
		outputPath := filepath.Join("piechart.html")
		if err4 := createInteractivePieChartHTML(phaseData, tpsData, outputPath); err4 != nil {
			fmt.Printf("生成HTML失败: %v\n", err4)
			os.Exit(1)
		}
		fmt.Printf("图表已保存至: %s\n", outputPath)
		return
	}

	// 根据调度器类型选择不同的解析函数
	switch *schedulerType {
	case "occ1":
		schedulerName = "OCC1"
		fmt.Printf("正在解析日志文件 (OCC1): %s\n", logFile)
		data, err = parseOcc1LogFile(logFile)
	case "occ2":
		schedulerName = "OCC2"
		fmt.Printf("正在解析日志文件 (OCC2): %s\n", logFile)
		data, err = parseOcc2LogFile(logFile)
	case "reorder":
		schedulerName = "Reorder"
		fmt.Printf("正在解析日志文件 (Reorder): %s\n", logFile)
		data, err = parseReorderLogFile(logFile)
	case "graph":
		schedulerName = "Graph"
		fmt.Printf("正在解析日志文件 (Graph): %s\n", logFile)
		data, err = parseGraphLogFile(logFile)
	case "aria":
		schedulerName = "Aria"
		fmt.Printf("正在解析日志文件 (Aria): %s\n", logFile)
		data, err = parseAriaLogFile(logFile)
	case "blockstm":
		schedulerName = "BlockSTM"
		fmt.Printf("正在解析日志文件 (BlockSTM): %s\n", logFile)
		data, err = parseBlockSTMLogFile(logFile)
	case "serial":
		schedulerName = "Serial"
		fmt.Printf("正在解析日志文件 (Serial): %s\n", logFile)
		data, err = parseSerialLogFile(logFile)
	default:
		schedulerName = "WRIA"
		fmt.Printf("正在解析日志文件 (WRIA): %s\n", logFile)
		data, err = parseWriaLogFileTps(logFile)
	}

	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	if len(data) == 0 {
		fmt.Println("警告: 未找到TPS数据")
		os.Exit(1)
	}

	fmt.Printf("成功解析 %d 条TPS记录\n\n", len(data))

	// 打印统计信息
	calculateStats(data)

	// 创建图表
	page := components.NewPage()
	page.AddCharts(
		createLineChart(data, schedulerName),
		createBarChart(data, schedulerName),
	)

	// 保存HTML文件
	outputPath := filepath.Join("tps_performance_analysis.html")
	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := page.Render(f); err != nil {
		fmt.Printf("渲染图表失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("图表已保存至: %s\n", outputPath)
	fmt.Println("请在浏览器中打开此HTML文件查看交互式图表")
	fmt.Println("\n分析完成!")
}
