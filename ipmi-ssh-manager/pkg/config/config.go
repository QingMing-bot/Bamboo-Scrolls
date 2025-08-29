package config

// 统一配置加载：后续可由 flags / env / 默认值组合。当前实现仅读取环境变量，
// 若 main 已有 flag，可在解析 flag 后调用 MergeFlags 覆盖。

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Config 保存运行时关键参数。
type Config struct {
	DataDir              string // 数据目录
	MaxParallel          int    // 并发上限 (<=0 不限制)
	HistoryRetentionDays int    // (保留字段，当前清理逻辑可选)
	HistoryMaxRows       int
	HistoryFlushInterval int
	HistoryBatchSize     int
	RemoteAPIBase        string // 远程 API 基址 (非空则启用 remote 模式)
	RemoteAPIToken       string // 静态 Token(示例)；真实应通过登录流程获取
}

var (
	once   sync.Once
	global *Config
)

// Load 读取全局配置（只初始化一次）。
// 环境变量：
//
//	IPMI_MODE          (tui|server) 默认 tui
//	IPMI_ADDR          监听地址 (默认 :8080)
//	IPMI_DATA_DIR      数据目录 (默认 data)
//	IPMI_MAX_PARALLEL  并发数 (整数, 默认 0 不限)
func Load() *Config {
	once.Do(func() {
		c := &Config{
			DataDir:              envOr("IPMI_DATA_DIR", "data"),
			MaxParallel:          envInt("IPMI_MAX_PARALLEL", 0),
			HistoryRetentionDays: envInt("IPMI_HISTORY_RETENTION_DAYS", 30),
			HistoryMaxRows:       envInt("IPMI_HISTORY_MAX_ROWS", 10000),
			HistoryFlushInterval: envInt("IPMI_HISTORY_FLUSH_INTERVAL", 2),
			HistoryBatchSize:     envInt("IPMI_HISTORY_BATCH_SIZE", 20),
			RemoteAPIBase:        envOr("IPMI_REMOTE_API_BASE", ""),
			RemoteAPIToken:       envOr("IPMI_REMOTE_API_TOKEN", ""),
		}
		_ = os.MkdirAll(c.DataDir, 0755)
		global = c
	})
	return global
}

// DBPath 返回 sqlite 文件路径。
func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "machines.db") }

// Helpers
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
