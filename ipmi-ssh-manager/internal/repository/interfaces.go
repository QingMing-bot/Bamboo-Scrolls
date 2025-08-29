package repository

import "github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"

// MachineRepoIface 抽象机器仓库（本地或远程）。
type MachineRepoIface interface {
	GetByIPMI(string) (domain.Machine, error)
	GetByIDs([]int64) ([]domain.Machine, error)
	ListAll() ([]domain.Machine, error)
	Save(*domain.Machine) error
	BulkUpsert([]domain.Machine) error
	DeleteByIPMI(string) error
	SearchByIPMI(string) ([]domain.Machine, error)
	EnsureSchema() error // 远程实现可为 no-op
}

// HistoryRepoIface 抽象历史仓库。
type HistoryRepoIface interface {
	Insert(*domain.ExecHistory) error
	ListRecent(int) ([]domain.ExecHistory, error)
	ListFiltered(int, string, string) ([]domain.ExecHistory, error)
	Cleanup(int, int) error
	EnsureSchema() error // 本地建表；远程 no-op
}

// 编译期断言本地实现满足接口
var _ MachineRepoIface = (*MachineRepo)(nil)
var _ HistoryRepoIface = (*HistoryRepo)(nil)
