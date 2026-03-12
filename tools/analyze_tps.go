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
	schedulerType := flag.String("type", "wria", "调度器类型: wria, occ1, occ2, reorder, graph 或 aria")
	flag.Parse()

	// 日志文件路径
	logPath := filepath.Join("build", "release", "chainmaker-v2.3.8-wx-org.chainmaker.org", "log", "system.log")

	var schedulerName string
	var data []TPSData
	var err error

	// 根据调度器类型选择不同的解析函数
	switch *schedulerType {
	case "occ1":
		schedulerName = "OCC1"
		fmt.Printf("正在解析日志文件 (OCC1): %s\n", logPath)
		data, err = parseOcc1LogFile(logPath)
	case "occ2":
		schedulerName = "OCC2"
		fmt.Printf("正在解析日志文件 (OCC2): %s\n", logPath)
		data, err = parseOcc2LogFile(logPath)
	case "reorder":
		schedulerName = "Reorder"
		fmt.Printf("正在解析日志文件 (Reorder): %s\n", logPath)
		data, err = parseReorderLogFile(logPath)
	case "graph":
		schedulerName = "Graph"
		fmt.Printf("正在解析日志文件 (Graph): %s\n", logPath)
		data, err = parseGraphLogFile(logPath)
	case "aria":
		schedulerName = "Aria"
		fmt.Printf("正在解析日志文件 (Aria): %s\n", logPath)
		data, err = parseAriaLogFile(logPath)
	default:
		schedulerName = "WRIA"
		fmt.Printf("正在解析日志文件 (WRIA): %s\n", logPath)
		data, err = parseWriaLogFileTps(logPath)
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
	outputPath := filepath.Join("tools", "tps_performance_analysis.html")
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
