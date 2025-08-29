package domain

type ExecTask struct {
	Command    string
	Timeout    int     // 秒
	MachineIDs []int64 // 目标机器ID列表
	Parallel   int     // 每任务并发(>0 覆盖全局)
	AuthMode   string  // "key"(默认) | "password"
	Password   string  // 当 AuthMode=="password" 时使用 (一次性，不落盘)
	Stream     bool    // 是否实时流式输出
}

type ExecResult struct {
	MachineID     int64
	IPMIIP        string
	SSHIP         string
	SSHUser       string
	Stdout        string
	Stderr        string
	ExitCode      int
	Err           error
	UsedGlobalKey bool // 当使用全局私钥回退时为 true
}
