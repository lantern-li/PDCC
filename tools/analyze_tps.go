package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

/*
使用方式：
go run tools/analyze_tps.go -type graph
*/
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
	Phase1Selection      float64 // ms
	Phase2Execution      float64
	Phase3Reordering     float64
	Phase4VersionTagging float64
	Phase5Merging        float64
	Phase6ConflictDetect float64
	Phase7Revalidation   float64
	Phase8Commit         float64
	Phase9TxReset        float64
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
			`phase3\(reordering\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase4\(versionTagging\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase5\(merging\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase6\(conflictDetection\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase7\(revalidation\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase8\(commit\)=([\d.]+(?:ns|µs|ms|s)) ` +
			`phase9\(txReset\)=([\d.]+(?:ns|µs|ms|s))`,
	)

	var data []PhaseTimeData
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		m := pattern.FindStringSubmatch(scanner.Text())
		if len(m) == 10 {
			data = append(data, PhaseTimeData{
				Phase1Selection:      parseDurationMs(m[1]),
				Phase2Execution:      parseDurationMs(m[2]),
				Phase3Reordering:     parseDurationMs(m[3]),
				Phase4VersionTagging: parseDurationMs(m[4]),
				Phase5Merging:        parseDurationMs(m[5]),
				Phase6ConflictDetect: parseDurationMs(m[6]),
				Phase7Revalidation:   parseDurationMs(m[7]),
				Phase8Commit:         parseDurationMs(m[8]),
				Phase9TxReset:        parseDurationMs(m[9]),
			})
		}
	}
	return data, scanner.Err()
}

// createPhasePieChart 创建9个阶段平均耗时饼图
func createPhasePieChart(data []PhaseTimeData) *charts.Pie {
	if len(data) == 0 {
		return nil
	}

	names := []string{
		"1.Selection", "2.Execution", "3.Reordering", "4.VersionTagging",
		"5.Merging", "6.ConflictDetection", "7.Revalidation", "8.Commit", "9.TxReset",
	}
	sums := make([]float64, 9)
	for _, d := range data {
		sums[0] += d.Phase1Selection
		sums[1] += d.Phase2Execution
		sums[2] += d.Phase3Reordering
		sums[3] += d.Phase4VersionTagging
		sums[4] += d.Phase5Merging
		sums[5] += d.Phase6ConflictDetect
		sums[6] += d.Phase7Revalidation
		sums[7] += d.Phase8Commit
		sums[8] += d.Phase9TxReset
	}
	n := float64(len(data))
	items := make([]opts.PieData, 9)
	for i := range names {
		avg := sums[i] / n
		items[i] = opts.PieData{Name: fmt.Sprintf("%s(%.3fms)", names[i], avg), Value: avg}
	}

	pie := charts.NewPie()
	pie.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "PDCC 各阶段平均耗时分布",
			Subtitle: fmt.Sprintf("基于 %d 个区块的统计", len(data)),
		}),
		charts.WithTooltipOpts(opts.Tooltip{Formatter: "{b}: {d}%"}),
		charts.WithLegendOpts(opts.Legend{Orient: "vertical", Right: "5%", Top: "20%"}),
	)
	pie.AddSeries("阶段耗时", items).
		SetSeriesOptions(charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Formatter: "{b}"}))
	return pie
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

	// wriaPieChart 单独处理：只解析阶段耗时并绘制饼图
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
		fmt.Printf("成功解析 %d 条阶段耗时记录\n", len(phaseData))
		page := components.NewPage()
		page.AddCharts(createPhasePieChart(phaseData))
		outputPath := filepath.Join("piechart.html")
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
