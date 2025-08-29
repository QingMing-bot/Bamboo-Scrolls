package ssh

// Deprecated: 请使用 exec.go 中的 ConnectionPool。
// 该文件保留占位，避免外部早期引用报错，可后续删除。

// Re-export 类型 (如果外部有引用 internal/ssh/pool.go 的计划)。
type Pool = ConnectionPool

func NewPool() *Pool { return NewConnectionPool() }
