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
	"time"
)

// StageData 存储各个阶段的耗时数据
type StageData struct {
	ExecutionStage                 []time.Duration
	DeterministicReorderStage      []time.Duration
	WriteSetMergingStage           []time.Duration
	CheckCommitAndRecheckingStage  []time.Duration
	ApplyWriteCacheToSnapshotStage []time.Duration
}

// parseDuration 解析 Go time.Duration 格式的字符串
// 支持格式: 1.234ms, 567.8µs, 1.234567s, 1m2.3s 等
func parseDuration(durationStr string) (time.Duration, error) {
	// 去除空格
	durationStr = strings.TrimSpace(durationStr)

	// 尝试直接解析
	duration, err := time.ParseDuration(durationStr)
	if err == nil {
		return duration, nil
	}

	// 如果直接解析失败，尝试处理一些特殊格式
	// 例如: "1.234ms" 可能需要转换
	return time.ParseDuration(durationStr)
}

// parseLogFile 从日志文件中解析各个阶段的耗时数据
func parseLogFileWria(logPath string) (*StageData, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer file.Close()

	data := &StageData{
		ExecutionStage:                 make([]time.Duration, 0),
		DeterministicReorderStage:      make([]time.Duration, 0),
		WriteSetMergingStage:           make([]time.Duration, 0),
		CheckCommitAndRecheckingStage:  make([]time.Duration, 0),
		ApplyWriteCacheToSnapshotStage: make([]time.Duration, 0),
	}

	// 定义正则表达式来匹配不同阶段的日志
	// 匹配 [ExecutionStage] execute XX txs finished, total cost=XXms
	executionPattern := regexp.MustCompile(`\[ExecutionStage\].*total cost=([^\s,]+)`)

	// 匹配 [deterministicReorderStage]: total cost=XXms
	reorderPattern := regexp.MustCompile(`\[deterministicReorderStage\]:\s*total cost=([^\s,]+)`)

	// 匹配 [writeSetMergingStage]: total cost=XXms
	mergingPattern := regexp.MustCompile(`\[writeSetMergingStage\]:\s*total cost=([^\s,]+)`)

	// 匹配 [checkCommitAndRecheckingStage]: total cost=XXms
	checkPattern := regexp.MustCompile(`\[checkCommitAndRecheckingStage\]:\s*total cost=([^\s,]+)`)

	// 匹配 [applyWriteCacheToSnapshotStage]: total cost=XXms
	applyPattern := regexp.MustCompile(`\[applyWriteCacheToSnapshotStage\]:\s*total cost=([^\s,]+)`)

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// 检查 ExecutionStage
		if matches := executionPattern.FindStringSubmatch(line); len(matches) == 2 {
			if duration, err := parseDuration(matches[1]); err == nil {
				data.ExecutionStage = append(data.ExecutionStage, duration)
			} else {
				fmt.Printf("警告: 第%d行无法解析ExecutionStage耗时: %s (错误: %v)\n", lineCount, matches[1], err)
			}
		}

		// 检查 DeterministicReorderStage
		if matches := reorderPattern.FindStringSubmatch(line); len(matches) == 2 {
			if duration, err := parseDuration(matches[1]); err == nil {
				data.DeterministicReorderStage = append(data.DeterministicReorderStage, duration)
			} else {
				fmt.Printf("警告: 第%d行无法解析DeterministicReorderStage耗时: %s (错误: %v)\n", lineCount, matches[1], err)
			}
		}

		// 检查 WriteSetMergingStage
		if matches := mergingPattern.FindStringSubmatch(line); len(matches) == 2 {
			if duration, err := parseDuration(matches[1]); err == nil {
				data.WriteSetMergingStage = append(data.WriteSetMergingStage, duration)
			} else {
				fmt.Printf("警告: 第%d行无法解析WriteSetMergingStage耗时: %s (错误: %v)\n", lineCount, matches[1], err)
			}
		}

		// 检查 CheckCommitAndRecheckingStage
		if matches := checkPattern.FindStringSubmatch(line); len(matches) == 2 {
			if duration, err := parseDuration(matches[1]); err == nil {
				data.CheckCommitAndRecheckingStage = append(data.CheckCommitAndRecheckingStage, duration)
			} else {
				fmt.Printf("警告: 第%d行无法解析CheckCommitAndRecheckingStage耗时: %s (错误: %v)\n", lineCount, matches[1], err)
			}
		}

		// 检查 ApplyWriteCacheToSnapshotStage
		if matches := applyPattern.FindStringSubmatch(line); len(matches) == 2 {
			if duration, err := parseDuration(matches[1]); err == nil {
				data.ApplyWriteCacheToSnapshotStage = append(data.ApplyWriteCacheToSnapshotStage, duration)
			} else {
				fmt.Printf("警告: 第%d行无法解析ApplyWriteCacheToSnapshotStage耗时: %s (错误: %v)\n", lineCount, matches[1], err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件错误: %w", err)
	}

	return data, nil
}

// calculateAverage 计算平均值
func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}

	return sum / time.Duration(len(durations))
}

// formatDuration 格式化耗时为可读字符串
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.2fns", float64(d.Nanoseconds()))
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000.0)
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// formatPercentage 计算并格式化百分比
func formatPercentage(duration, total time.Duration) string {
	if total == 0 {
		return "0.00%"
	}
	percentage := float64(duration) / float64(total) * 100
	return fmt.Sprintf("%.2f%%", percentage)
}

// printStatistics 打印统计信息
func printStatistics(data *StageData) {
	fmt.Println("\n============================================================")
	fmt.Println("WRIA 算法各阶段耗时统计")
	fmt.Println("============================================================\n")

	// 计算各阶段的平均值
	avgExecution := calculateAverage(data.ExecutionStage)
	avgReorder := calculateAverage(data.DeterministicReorderStage)
	avgMerging := calculateAverage(data.WriteSetMergingStage)
	avgCheck := calculateAverage(data.CheckCommitAndRecheckingStage)
	avgApply := calculateAverage(data.ApplyWriteCacheToSnapshotStage)

	// 计算总平均耗时
	totalAvg := avgExecution + avgReorder + avgMerging + avgCheck + avgApply

	// 打印详细统计
	fmt.Printf("%-40s 样本数: %-6d 平均耗时: %-12s 占比: %s\n",
		"ExecutionStage:",
		len(data.ExecutionStage),
		formatDuration(avgExecution),
		formatPercentage(avgExecution, totalAvg))

	fmt.Printf("%-40s 样本数: %-6d 平均耗时: %-12s 占比: %s\n",
		"DeterministicReorderStage:",
		len(data.DeterministicReorderStage),
		formatDuration(avgReorder),
		formatPercentage(avgReorder, totalAvg))

	fmt.Printf("%-40s 样本数: %-6d 平均耗时: %-12s 占比: %s\n",
		"WriteSetMergingStage:",
		len(data.WriteSetMergingStage),
		formatDuration(avgMerging),
		formatPercentage(avgMerging, totalAvg))

	fmt.Printf("%-40s 样本数: %-6d 平均耗时: %-12s 占比: %s\n",
		"CheckCommitAndRecheckingStage:",
		len(data.CheckCommitAndRecheckingStage),
		formatDuration(avgCheck),
		formatPercentage(avgCheck, totalAvg))

	fmt.Printf("%-40s 样本数: %-6d 平均耗时: %-12s 占比: %s\n",
		"ApplyWriteCacheToSnapshotStage:",
		len(data.ApplyWriteCacheToSnapshotStage),
		formatDuration(avgApply),
		formatPercentage(avgApply, totalAvg))

	fmt.Println("\n------------------------------------------------------------")
	fmt.Printf("总平均耗时: %s\n", formatDuration(totalAvg))
	fmt.Println("============================================================\n")

	// 打印详细数据（可选，方便调试）
	if len(data.ExecutionStage) > 0 {
		fmt.Println("各阶段详细耗时范围:")
		printDetailedStats("ExecutionStage", data.ExecutionStage)
		printDetailedStats("DeterministicReorderStage", data.DeterministicReorderStage)
		printDetailedStats("WriteSetMergingStage", data.WriteSetMergingStage)
		printDetailedStats("CheckCommitAndRecheckingStage", data.CheckCommitAndRecheckingStage)
		printDetailedStats("ApplyWriteCacheToSnapshotStage", data.ApplyWriteCacheToSnapshotStage)
		fmt.Println("============================================================\n")
	}
}

// printDetailedStats 打印详细统计信息（最小值、最大值）
func printDetailedStats(stageName string, durations []time.Duration) {
	if len(durations) == 0 {
		fmt.Printf("  %-35s 无数据\n", stageName+":")
		return
	}

	min := durations[0]
	max := durations[0]

	for _, d := range durations {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	fmt.Printf("  %-35s 最小: %-12s 最大: %-12s\n",
		stageName+":",
		formatDuration(min),
		formatDuration(max))
}

// convertToCSV 将数据转换为CSV格式并保存
func convertToCSV(data *StageData, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("无法创建CSV文件: %w", err)
	}
	defer file.Close()

	// 写入CSV头
	file.WriteString("Index,ExecutionStage(ns),DeterministicReorderStage(ns),WriteSetMergingStage(ns),CheckCommitAndRecheckingStage(ns),ApplyWriteCacheToSnapshotStage(ns)\n")

	// 找到最大长度
	maxLen := len(data.ExecutionStage)
	if len(data.DeterministicReorderStage) > maxLen {
		maxLen = len(data.DeterministicReorderStage)
	}
	if len(data.WriteSetMergingStage) > maxLen {
		maxLen = len(data.WriteSetMergingStage)
	}
	if len(data.CheckCommitAndRecheckingStage) > maxLen {
		maxLen = len(data.CheckCommitAndRecheckingStage)
	}
	if len(data.ApplyWriteCacheToSnapshotStage) > maxLen {
		maxLen = len(data.ApplyWriteCacheToSnapshotStage)
	}

	// 写入数据
	for i := 0; i < maxLen; i++ {
		file.WriteString(strconv.Itoa(i + 1))
		file.WriteString(",")

		if i < len(data.ExecutionStage) {
			file.WriteString(strconv.FormatInt(data.ExecutionStage[i].Nanoseconds(), 10))
		}
		file.WriteString(",")

		if i < len(data.DeterministicReorderStage) {
			file.WriteString(strconv.FormatInt(data.DeterministicReorderStage[i].Nanoseconds(), 10))
		}
		file.WriteString(",")

		if i < len(data.WriteSetMergingStage) {
			file.WriteString(strconv.FormatInt(data.WriteSetMergingStage[i].Nanoseconds(), 10))
		}
		file.WriteString(",")

		if i < len(data.CheckCommitAndRecheckingStage) {
			file.WriteString(strconv.FormatInt(data.CheckCommitAndRecheckingStage[i].Nanoseconds(), 10))
		}
		file.WriteString(",")

		if i < len(data.ApplyWriteCacheToSnapshotStage) {
			file.WriteString(strconv.FormatInt(data.ApplyWriteCacheToSnapshotStage[i].Nanoseconds(), 10))
		}

		file.WriteString("\n")
	}

	return nil
}

func main() {
	// 命令行参数
	logPath := flag.String("log", "", "日志文件路径 (默认: build/release/chainmaker-v2.3.8-wx-org.chainmaker.org/log/system.log)")
	csvOutput := flag.String("csv", "", "CSV输出文件路径 (可选)")
	flag.Parse()

	// 确定日志文件路径
	var finalLogPath string
	if *logPath == "" {
		finalLogPath = filepath.Join("build", "release", "chainmaker-v2.3.8-wx-org.chainmaker.org", "log", "system.log")
	} else {
		finalLogPath = *logPath
	}

	fmt.Printf("正在解析日志文件: %s\n", finalLogPath)

	// 解析日志文件
	data, err := parseLogFileWria(finalLogPath)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 检查是否有数据
	totalSamples := len(data.ExecutionStage) + len(data.DeterministicReorderStage) +
		len(data.WriteSetMergingStage) + len(data.CheckCommitAndRecheckingStage) +
		len(data.ApplyWriteCacheToSnapshotStage)

	if totalSamples == 0 {
		fmt.Println("\n警告: 未找到任何WRIA阶段的耗时数据")
		fmt.Println("请确保:")
		fmt.Println("  1. 日志文件路径正确")
		fmt.Println("  2. ChainMaker 正在使用 WRIA 调度器")
		fmt.Println("  3. 日志级别设置为 DEBUG (能够输出 DebugDynamic 日志)")
		os.Exit(1)
	}

	fmt.Printf("成功解析 %d 条耗时记录\n", totalSamples)

	// 打印统计信息
	printStatistics(data)

	// 如果指定了CSV输出路径，则保存CSV
	if *csvOutput != "" {
		if err := convertToCSV(data, *csvOutput); err != nil {
			fmt.Printf("保存CSV文件失败: %v\n", err)
		} else {
			fmt.Printf("CSV数据已保存至: %s\n", *csvOutput)
		}
	}

	fmt.Println("分析完成!")
}
