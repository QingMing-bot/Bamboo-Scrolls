package domain

import "time"

// ExecHistory 记录单次命令在某台机器的执行结果
type ExecHistory struct {
	ID         int64     `json:"id"`
	MachineID  int64     `json:"machine_id"`
	IPMIIP     string    `json:"ipmi_ip"`
	Command    string    `json:"command"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	ExitCode   int       `json:"exit_code"`
	ErrorText  string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
}
