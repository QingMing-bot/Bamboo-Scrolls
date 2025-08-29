## IPMI SSH Manager (Wails 桌面版)

面向多台服务器的批量 SSH 命令执行与历史记录管理桌面应用。采用 Go + SQLite + Wails，单可执行文件交付，适合运维批量巡检与快速指令分发场景。

### 功能特性
* 资产管理：添加 / 更新 / 删除 / 批量导入
* 批量命令执行：
  * 一次性聚合结果
  * 流式实时输出 (事件 `exec_result`)
  * Job 模式（可取消，结束事件 `exec_job_done`）
  * 进度百分比 (progress 0.0~1.0)
* 并发 + 超时：全局配置 + 单任务覆盖
* 历史记录：异步批量写入、筛选、自动刷新、按天 + 行数保留策略定期清理
* 导入 / 导出：JSON / CSV，支持 SSH Key 脱敏导出
* SSH Key 加密存储：Windows 使用 DPAPI 加密（其它平台当前回退为明文，后续增强）
* 事件驱动：前端无需轮询即可获取执行流
* 单文件内嵌 UI：`webui/index.html` 直接 embed，启动即用
* CI 工作流：构建 + 测试（GitHub Actions）

### 快速开始
```powershell
git clone <repo>
cd ipmi-ssh-manager
go build -o ipmi-ssh-manager.exe ./cmd/app
./ipmi-ssh-manager.exe
```

或使用最小构建脚本：
```powershell
./build.ps1           # 标准构建 (bin/ 输出)
./build.ps1 -Wails    # 若已安装 wails CLI，产出带图标/资源的构建
./build.ps1 -Race     # 启用 -race
./build.ps1 -Clean    # 清理 bin/ dist/
```

### 运行
运行生成的可执行文件；首次启动会在 `data/` 下创建 `machines.db`。支持通过环境变量调整并发 / 历史策略。

### 配置 (环境变量)
| 变量 | 说明 | 默认 |
|------|------|------|
| IPMI_DATA_DIR | 数据目录 | data |
| IPMI_MAX_PARALLEL | 全局并发上限 (<=0 不限制) | 0 |
| IPMI_HISTORY_RETENTION_DAYS | 历史按天清理 (<=0 不按天删) | 30 |
| IPMI_HISTORY_MAX_ROWS | 历史最大行数 (超出裁剪旧数据) | 10000 |
| IPMI_HISTORY_FLUSH_INTERVAL | 历史写入批量 flush 秒 | 2 |
| IPMI_HISTORY_BATCH_SIZE | 批量写入最大条数 | 20 |

### 数据库 Schema
应用启动自动确保：
```sql
CREATE TABLE IF NOT EXISTS machines (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ipmi_ip TEXT NOT NULL UNIQUE,
  ssh_ip TEXT,
  ssh_user TEXT,
  ssh_key TEXT,
  remark TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS exec_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  machine_id INTEGER,
  ipmi_ip TEXT,
  command TEXT,
  stdout TEXT,
  stderr TEXT,
  exit_code INTEGER,
  error_text TEXT,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  duration_ms INTEGER
);
```

### 目录结构
```
cmd/app/main.go          # 应用入口
internal/domain/         # 领域模型 (Machine, ExecHistory, ExecTask ...)
internal/repository/     # 数据访问 (MachineRepo, HistoryRepo)
internal/service/        # 执行调度 / 异步历史写入 / 任务管理
internal/ssh/            # SSH 执行器 & 连接池 + 测试 Mock
internal/wailsapi/       # Wails 绑定 & 事件发射
pkg/config/              # 配置加载
pkg/importexport/        # JSON / CSV 导入导出与脱敏
pkg/secret/              # SSH Key 加解密适配层 (Windows DPAPI)
webui/                   # 内嵌前端 (index.html + embed.go)
build.ps1                # 最小构建脚本
.github/workflows/ci.yml # CI 配置
```

### 开发者提示
* 事件：
  * 单次/流式执行：`exec_result` (字段含 `ipmi_ip` / `stdout` / `stderr` / `exit_code` / `error` / `progress`)
  * 任务结束：`exec_job_done` (字段 `job_id`)
* 取消任务：`CancelJob(jobID)`
* 导出脱敏：`ExportMachines(format, true)` 清除 SSH Key
* 历史清理：main 中每小时调用一次 `HistoryRepo.Cleanup()`
* 进度计算：完成数 / 总数 (浮点 0~1)，前端示例已输出百分比
* SSH Key 加密：保存时自动加密（Windows），读取自动解密；非 Windows 暂为明文（带 `enc:` 前缀的数据在非 Windows 读取会失败）

### 未来改进路线
1. 剩余时间 / ETA 预估
2. UI 资源拆分与构建管线（模块化 JS/CSS）
3. 跨平台统一安全存储 (macOS Keychain / Linux Secret Service)
4. 执行模板与收藏功能
5. 多跳 / 代理执行 (Bastion / Jump Host)
6. 历史过滤增强：exit_code / 时间范围 / 关键字高亮
7. Release 自动化：多平台产物 + 版本元数据
8. 更完整测试覆盖 (执行中断 / 大并发 / 数据迁移)

### 迁移说明
早期版本包含 TUI 与 HTTP Server 模式，已完全移除；如需回溯请查看历史提交。`frontend/` React 原型与旧多模式 build 脚本均已废弃。

### 测试
执行：
```powershell
go test ./...
```
现有测试示例：`internal/service/exec_service_test.go` 使用 MockExecutor 验证批量执行与历史落库。

### 构建脚本使用示例
```powershell
# 标准构建 (输出 bin/ipmi-ssh-manager(.exe))
./build.ps1

# 启用 -race
./build.ps1 -Race

# 触发 Wails GUI 构建 (需要已安装 wails CLI)
./build.ps1 -Wails

# 清理输出目录
./build.ps1 -Clean
```

### License
MIT，详见根目录 `LICENSE`。


