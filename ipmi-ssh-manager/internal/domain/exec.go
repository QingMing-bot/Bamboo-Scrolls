package domain

type ExecTask struct {
	Command    string
	Timeout    int     // 秒
	MachineIDs []int64 // 目标机器ID列表
	Parallel   int     // 每任务并发(>0 覆盖全局)
}

type ExecResult struct {
	MachineID int64
	IPMIIP    string
	SSHIP     string
	SSHUser   string
	Stdout    string
	Stderr    string
	ExitCode  int
	Err       error
}
