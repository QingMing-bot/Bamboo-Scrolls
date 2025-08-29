package wailsapi

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/repository"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/service"
	"github.com/QingMing-Bot/ipmi-ssh-manager/pkg/importexport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func errToString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// Backend 暴露给 Wails 前端的绑定对象
type Backend struct {
	db      *sql.DB
	repo    *repository.MachineRepo
	hRepo   *repository.HistoryRepo
	execSvc *service.ExecService
	ctx     context.Context // wails runtime context for events
}

func NewBackend(db *sql.DB, repo *repository.MachineRepo, hRepo *repository.HistoryRepo, execSvc *service.ExecService) *Backend {
	return &Backend{db: db, repo: repo, hRepo: hRepo, execSvc: execSvc}
}

// ListMachines 全量列表
func (b *Backend) ListMachines() ([]domain.Machine, error) { return b.repo.ListAll() }

// UpsertMachine 保存或更新
func (b *Backend) UpsertMachine(m domain.Machine) error { return b.repo.Save(&m) }

// DeleteMachine 删除
func (b *Backend) DeleteMachine(ipmi string) error { return b.repo.DeleteByIPMI(ipmi) }

// Execute 批量执行
func (b *Backend) Execute(command string, ids []int64, timeoutSec int, parallel int) ([]domain.ExecResult, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return b.execSvc.BatchExec(domain.ExecTask{Command: command, Timeout: timeoutSec, MachineIDs: ids, Parallel: parallel})
}

// ExecuteStream 逐个返回结果: 前端可轮询或未来通过事件机制。
func (b *Backend) ExecuteStream(command string, ids []int64, timeoutSec int, parallel int) ([]domain.ExecResult, error) {
	var out []domain.ExecResult
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	err := b.execSvc.StreamExec(domain.ExecTask{Command: command, Timeout: timeoutSec, MachineIDs: ids, Parallel: parallel}, func(r domain.ExecResult) {
		out = append(out, r)
	})
	return out, err
}

// SetCtx 在 OnStartup 时注入 wails context
func (b *Backend) SetCtx(ctx context.Context) { b.ctx = ctx }

// ExecuteStreamEvents 使用事件逐条推送结果 (事件名: exec_result)
// 前端： runtime.EventsOn("exec_result", cb)
func (b *Backend) ExecuteStreamEvents(command string, ids []int64, timeoutSec int, parallel int) error {
	if b.ctx == nil {
		// 退化为聚合返回
		_, err := b.ExecuteStream(command, ids, timeoutSec, parallel)
		return err
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	total := len(ids)
	var done int64
	return b.execSvc.StreamExec(domain.ExecTask{Command: command, Timeout: timeoutSec, MachineIDs: ids, Parallel: parallel}, func(r domain.ExecResult) {
		done++
		payload := map[string]any{
			"machine_id": r.MachineID,
			"ipmi_ip":    r.IPMIIP,
			"ssh_ip":     r.SSHIP,
			"ssh_user":   r.SSHUser,
			"stdout":     r.Stdout,
			"stderr":     r.Stderr,
			"exit_code":  r.ExitCode,
			"error":      "",
			"progress":   float64(done) / float64(total),
		}
		if r.Err != nil {
			payload["error"] = r.Err.Error()
		}
		runtime.EventsEmit(b.ctx, "exec_result", payload)
	})
}

// StartJob 启动带 jobID 的流执行 (事件推送)；返回 jobID
func (b *Backend) StartJob(jobID string, command string, ids []int64, timeoutSec int, parallel int) (string, error) {
	if b.ctx == nil {
		return "", errors.New("context not ready")
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	total := len(ids)
	var done int64
	jid, err := b.execSvc.StartBatch(jobID, domain.ExecTask{Command: command, Timeout: timeoutSec, MachineIDs: ids, Parallel: parallel}, func(r domain.ExecResult) {
		done++
		payload := map[string]any{
			"job_id":     jobID,
			"machine_id": r.MachineID,
			"ipmi_ip":    r.IPMIIP,
			"stdout":     r.Stdout,
			"stderr":     r.Stderr,
			"exit_code":  r.ExitCode,
			"error":      errToString(r.Err),
			"progress":   float64(done) / float64(total),
		}
		runtime.EventsEmit(b.ctx, "exec_result", payload)
	})
	if err == nil {
		go func(id string) {
			// 轮询等待任务结束
			for {
				select {
				case <-time.After(300 * time.Millisecond):
					if !b.execSvc.HasJob(id) { // 已结束
						runtime.EventsEmit(b.ctx, "exec_job_done", map[string]any{"job_id": id})
						return
					}
				}
			}
		}(jid)
	}
	return jid, err
}

// CancelJob 取消指定 job
func (b *Backend) CancelJob(jobID string) bool { return b.execSvc.Cancel(jobID) }

// RecentHistory 最近历史
func (b *Backend) RecentHistory(limit int) ([]domain.ExecHistory, error) {
	return b.hRepo.ListRecent(limit)
}

// RecentHistoryFiltered 过滤历史
func (b *Backend) RecentHistoryFiltered(limit int, ipmi, cmd string) ([]domain.ExecHistory, error) {
	return b.hRepo.ListFiltered(limit, ipmi, cmd)
}

// ImportMachines 导入 (format=json|csv)
func (b *Backend) ImportMachines(data string, format string) (int, error) {
	var ms []domain.Machine
	var err error
	if format == "csv" {
		ms, err = importexport.ParseMachinesCSV([]byte(data))
	} else {
		ms, err = importexport.ParseMachinesJSON([]byte(data))
	}
	if err != nil {
		return 0, err
	}
	if err = b.repo.BulkUpsert(ms); err != nil {
		return 0, err
	}
	return len(ms), nil
}

// ExportMachines 导出 (format=json|csv)
// ExportMachines 支持脱敏选项 redact=true 去除 ssh_key
func (b *Backend) ExportMachines(format string, redact bool) (string, error) {
	list, err := b.repo.ListAll()
	if err != nil {
		return "", err
	}
	if redact {
		for i := range list {
			list[i].SSHKey = "" // 去掉敏感
		}
	}
	if format == "csv" {
		return importexport.RenderMachinesCSV(list), nil
	}
	return importexport.SerializeMachinesJSON(list)
}

// Shutdown 钩子
func (b *Backend) Shutdown(ctx context.Context) error { return nil }
