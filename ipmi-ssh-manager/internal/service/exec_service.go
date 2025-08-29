package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/repository"
)

// SSHExecutor 抽象执行接口，便于替换真实 SSH / Mock
type SSHExecutor interface {
	Exec(ctx context.Context, user, addr, authMode, keyOrPass, cmd string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
}

// 可选: 若底层实现支持流式输出，可实现该接口
type SSHStreamExecutor interface {
	SSHExecutor
	StreamExec(ctx context.Context, user, addr, authMode, keyOrPass, cmd string, timeout time.Duration, onChunk func([]byte, bool)) (stdout, stderr string, exitCode int, err error)
}

// ExecService 负责批量执行编排
type ExecService struct {
	repo              repository.MachineRepoIface
	hWriter           *HistoryWriter
	executor          SSHExecutor
	maxParallel       int
	mu                sync.Mutex
	jobs              map[string]context.CancelFunc
	globalKeyProvider func() string
}

func NewExecService(repo repository.MachineRepoIface, writer *HistoryWriter, executor SSHExecutor, maxParallel int) *ExecService {
	return &ExecService{repo: repo, hWriter: writer, executor: executor, maxParallel: maxParallel, jobs: make(map[string]context.CancelFunc)}
}

// SetGlobalKeyProvider 设置获取全局私钥的函数（避免直接依赖 Backend 造成循环）
func (s *ExecService) SetGlobalKeyProvider(f func() string) { s.globalKeyProvider = f }

// StartBatch 启动一个带 jobID 的流批执行，返回 jobID（若传入为空则自动生成）。
// 使用 StreamExec 语义（回调逐条）。
func (s *ExecService) StartBatch(jobID string, task domain.ExecTask, cb func(domain.ExecResult)) (string, error) {
	if jobID == "" {
		jobID = time.Now().Format("20060102_150405.000")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.jobs[jobID] = cancel
	s.mu.Unlock()
	go func() {
		_ = s.StreamExecWithCtx(ctx, task, cb)
		s.mu.Lock()
		delete(s.jobs, jobID)
		s.mu.Unlock()
	}()
	return jobID, nil
}

// Cancel 取消指定 jobID
func (s *ExecService) Cancel(jobID string) bool {
	s.mu.Lock()
	cancel, ok := s.jobs[jobID]
	s.mu.Unlock()
	if ok {
		cancel()
		return true
	}
	return false
}

// HasJob 判断 job 是否仍在运行
func (s *ExecService) HasJob(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jobs[jobID]
	return ok
}

func errToString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// BatchExec 批量执行命令
// 传入 ExecTask：Command / Timeout(s) / MachineIDs
func (s *ExecService) BatchExec(task domain.ExecTask) ([]domain.ExecResult, error) {
	if task.Command == "" {
		return nil, errors.New("command empty")
	}
	if len(task.MachineIDs) == 0 {
		return nil, errors.New("no machines")
	}
	if task.Timeout <= 0 {
		task.Timeout = 30
	}
	timeout := time.Duration(task.Timeout) * time.Second

	// 取机器
	machines, err := s.repo.GetByIDs(task.MachineIDs)
	if err != nil {
		return nil, err
	}
	mMap := make(map[int64]domain.Machine, len(machines))
	for _, m := range machines {
		mMap[int64(m.ID)] = m
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []domain.ExecResult
		sem     chan struct{}
	)
	limit := s.maxParallel
	if task.Parallel > 0 {
		limit = task.Parallel
	}
	if limit > 0 {
		sem = make(chan struct{}, limit)
	}
	add := func(r domain.ExecResult) {
		mu.Lock()
		results = append(results, r)
		mu.Unlock()
	}

	for _, id := range task.MachineIDs {
		m, ok := mMap[id]
		if !ok {
			add(domain.ExecResult{MachineID: id, Err: errors.New("machine not found")})
			continue
		}
		if sem != nil {
			sem <- struct{}{}
		}
		wg.Add(1)
		go func(mc domain.Machine) {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			authMode := task.AuthMode
			if authMode == "" {
				authMode = "key"
			}
			secret := mc.SSHKey
			usedGlobal := false
			if authMode == "key" && secret == "" && s.globalKeyProvider != nil { // fallback 到全局 key
				secret = s.globalKeyProvider()
				if secret != "" {
					usedGlobal = true
				}
			}
			if authMode == "password" {
				secret = task.Password
			}
			stdout, stderr, code, exErr := s.executor.Exec(ctx, mc.SSHUser, mc.SSHIP, authMode, secret, task.Command, timeout)
			finish := time.Now()
			r := domain.ExecResult{
				MachineID:     int64(mc.ID),
				IPMIIP:        mc.IPMIIP,
				SSHIP:         mc.SSHIP,
				SSHUser:       mc.SSHUser,
				Stdout:        stdout,
				Stderr:        stderr,
				ExitCode:      code,
				Err:           exErr,
				UsedGlobalKey: usedGlobal,
			}
			add(r)
			if s.hWriter != nil {
				h := domain.ExecHistory{
					MachineID:  int64(mc.ID),
					IPMIIP:     mc.IPMIIP,
					Command:    task.Command,
					Stdout:     stdout,
					Stderr:     stderr,
					ExitCode:   code,
					ErrorText:  errToString(exErr),
					StartedAt:  start,
					FinishedAt: finish,
					DurationMs: finish.Sub(start).Milliseconds(),
				}
				s.hWriter.Write(h)
			}
		}(m)
	}

	wg.Wait()
	return results, nil
}

// StreamExec 与 BatchExec 类似，但每个结果完成后通过回调立即返回。
// 回调需快速返回，避免阻塞整体执行；如需耗时操作可在回调内再开 goroutine。
func (s *ExecService) StreamExec(task domain.ExecTask, cb func(domain.ExecResult)) error {
	return s.StreamExecWithCtx(context.Background(), task, cb)
}

// StreamExecWithCtx 支持外部 context 取消
func (s *ExecService) StreamExecWithCtx(ctx context.Context, task domain.ExecTask, cb func(domain.ExecResult)) error {
	if task.Command == "" {
		return errors.New("command empty")
	}
	if len(task.MachineIDs) == 0 {
		return errors.New("no machines")
	}
	if task.Timeout <= 0 {
		task.Timeout = 30
	}
	timeout := time.Duration(task.Timeout) * time.Second
	machines, err := s.repo.GetByIDs(task.MachineIDs)
	if err != nil {
		return err
	}
	mMap := make(map[int64]domain.Machine, len(machines))
	for _, m := range machines {
		mMap[int64(m.ID)] = m
	}
	var wg sync.WaitGroup
	var sem chan struct{}
	limit := s.maxParallel
	if task.Parallel > 0 {
		limit = task.Parallel
	}
	if limit > 0 {
		sem = make(chan struct{}, limit)
	}
	for _, id := range task.MachineIDs {
		mc, ok := mMap[id]
		if !ok {
			cb(domain.ExecResult{MachineID: id, Err: errors.New("machine not found")})
			continue
		}
		if sem != nil {
			sem <- struct{}{}
		}
		wg.Add(1)
		go func(m domain.Machine) {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}
			start := time.Now()
			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			authMode := task.AuthMode
			if authMode == "" {
				authMode = "key"
			}
			secret := m.SSHKey
			usedGlobal := false
			if authMode == "key" && secret == "" && s.globalKeyProvider != nil { // fallback
				secret = s.globalKeyProvider()
				if secret != "" {
					usedGlobal = true
				}
			}
			if authMode == "password" {
				secret = task.Password
			}
			stdout, stderr, code, exErr := s.executor.Exec(cctx, m.SSHUser, m.SSHIP, authMode, secret, task.Command, timeout)
			finish := time.Now()
			res := domain.ExecResult{MachineID: int64(m.ID), IPMIIP: m.IPMIIP, SSHIP: m.SSHIP, SSHUser: m.SSHUser, Stdout: stdout, Stderr: stderr, ExitCode: code, Err: exErr, UsedGlobalKey: usedGlobal}
			cb(res)
			if s.hWriter != nil {
				s.hWriter.Write(domain.ExecHistory{MachineID: int64(m.ID), IPMIIP: m.IPMIIP, Command: task.Command, Stdout: stdout, Stderr: stderr, ExitCode: code, ErrorText: errToString(exErr), StartedAt: start, FinishedAt: finish, DurationMs: finish.Sub(start).Milliseconds()})
			}
		}(mc)
	}
	wg.Wait()
	return nil
}

// 单机实时流执行帮助：返回完整结果并在过程中使用 chunkCb 回调
func (s *ExecService) SingleStream(ctx context.Context, m domain.Machine, task domain.ExecTask, secret, authMode string, chunkCb func(int64, []byte, bool)) (domain.ExecResult, error) {
	timeout := time.Duration(task.Timeout) * time.Second
	start := time.Now()
	usedGlobal := false
	if authMode == "key" && secret == "" && s.globalKeyProvider != nil {
		secret = s.globalKeyProvider()
		if secret != "" {
			usedGlobal = true
		}
	}
	if authMode == "password" {
		secret = task.Password
	}
	var stdout, stderr string
	var code int
	var exErr error
	if se, ok := s.executor.(SSHStreamExecutor); ok { // 流式
		so, er, c, e := se.StreamExec(ctx, m.SSHUser, m.SSHIP, authMode, secret, task.Command, timeout, func(b []byte, isErr bool) {
			if chunkCb != nil {
				chunkCb(int64(m.ID), b, isErr)
			}
		})
		stdout, stderr, code, exErr = so, er, c, e
	} else { // 回退
		so, er, c, e := s.executor.Exec(ctx, m.SSHUser, m.SSHIP, authMode, secret, task.Command, timeout)
		stdout, stderr, code, exErr = so, er, c, e
	}
	finish := time.Now()
	res := domain.ExecResult{MachineID: int64(m.ID), IPMIIP: m.IPMIIP, SSHIP: m.SSHIP, SSHUser: m.SSHUser, Stdout: stdout, Stderr: stderr, ExitCode: code, Err: exErr, UsedGlobalKey: usedGlobal}
	if s.hWriter != nil {
		s.hWriter.Write(domain.ExecHistory{MachineID: int64(m.ID), IPMIIP: m.IPMIIP, Command: task.Command, Stdout: stdout, Stderr: stderr, ExitCode: code, ErrorText: errToString(exErr), StartedAt: start, FinishedAt: finish, DurationMs: finish.Sub(start).Milliseconds()})
	}
	return res, nil
}
