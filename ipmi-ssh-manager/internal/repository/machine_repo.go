package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/pkg/secret"
)

type MachineRepo struct {
	db *sql.DB
}

func NewMachineRepo(db *sql.DB) *MachineRepo {
	return &MachineRepo{db: db}
}

// ensureSchema 可以在应用启动时调用以补齐缺失列 (幂等)。
func (r *MachineRepo) EnsureSchema() error {
	// 添加缺失列 (sqlite 不支持 IF NOT EXISTS 列，需捕获错误忽略)
	alterStatements := []string{
		"ALTER TABLE machines ADD COLUMN ssh_key TEXT",
		"ALTER TABLE machines ADD COLUMN remark TEXT",
		"ALTER TABLE machines ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP",
	}
	for _, sql := range alterStatements {
		if _, err := r.db.Exec(sql); err != nil {
			// 如果是列已存在错误，忽略 (通过错误消息判断)
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate") &&
				!strings.Contains(strings.ToLower(err.Error()), "exists") &&
				!strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				// sqlite3 驱动会返回类似: duplicate column name: xxx
				// modernc.org/sqlite 返回: duplicate column name
				// 其它错误才返回
				// 也可能是 no such table: machines -> 需要上层先建表
				if strings.Contains(err.Error(), "no such table") {
					return errors.New("table 'machines' missing; create it before ensureSchema")
				}
			}
		}
	}
	return nil
}

func (r *MachineRepo) SearchByIPMI(ip string) ([]domain.Machine, error) {
	if ip == "" {
		ip = "%"
	} else {
		ip = "%" + ip + "%"
	}
	rows, err := r.db.Query(`SELECT id, ipmi_ip, ssh_ip, ssh_user, COALESCE(ssh_key,''), COALESCE(remark,''), COALESCE(created_at,'') FROM machines WHERE ipmi_ip LIKE ? ORDER BY id DESC`, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Machine
	for rows.Next() {
		var m domain.Machine
		var createdAtStr string
		if err := rows.Scan(&m.ID, &m.IPMIIP, &m.SSHIP, &m.SSHUser, &m.SSHKey, &m.Remark, &createdAtStr); err != nil {
			return nil, err
		}
		if createdAtStr != "" {
			// 尝试多种格式
			if ts, e := time.Parse(time.RFC3339Nano, createdAtStr); e == nil {
				m.CreatedAt = ts
			}
		}
		list = append(list, m)
	}
	return list, nil
}

func (r *MachineRepo) GetByIPMI(ip string) (domain.Machine, error) {
	var m domain.Machine
	row := r.db.QueryRow(`SELECT id, ipmi_ip, ssh_ip, ssh_user, COALESCE(ssh_key,''), COALESCE(remark,''), COALESCE(created_at,'') FROM machines WHERE ipmi_ip = ? LIMIT 1`, ip)
	var createdAtStr string
	if err := row.Scan(&m.ID, &m.IPMIIP, &m.SSHIP, &m.SSHUser, &m.SSHKey, &m.Remark, &createdAtStr); err != nil {
		return domain.Machine{}, err
	}
	if createdAtStr != "" {
		if ts, e := time.Parse(time.RFC3339Nano, createdAtStr); e == nil {
			m.CreatedAt = ts
		}
	}
	if m.SSHKey != "" { // 解密（忽略错误，保持兼容）
		if p, err := secret.DecryptString(m.SSHKey); err == nil && p != "" {
			m.SSHKey = p
		}
	}
	return m, nil
}

func (r *MachineRepo) GetByIDs(ids []int64) ([]domain.Machine, error) {
	if len(ids) == 0 {
		return []domain.Machine{}, nil
	}
	// 构建占位符
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT id, ipmi_ip, ssh_ip, ssh_user, COALESCE(ssh_key,''), COALESCE(remark,''), COALESCE(created_at,'') FROM machines WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Machine
	for rows.Next() {
		var m domain.Machine
		var createdAtStr string
		if err := rows.Scan(&m.ID, &m.IPMIIP, &m.SSHIP, &m.SSHUser, &m.SSHKey, &m.Remark, &createdAtStr); err != nil {
			return nil, err
		}
		if createdAtStr != "" {
			if ts, e := time.Parse(time.RFC3339Nano, createdAtStr); e == nil {
				m.CreatedAt = ts
			}
		}
		if m.SSHKey != "" {
			if p, err := secret.DecryptString(m.SSHKey); err == nil && p != "" {
				m.SSHKey = p
			}
		}
		list = append(list, m)
	}
	return list, nil
}

func (r *MachineRepo) Save(m *domain.Machine) error {
	// 插入或更新 (通过唯一 ipmi_ip 约束实现 upsert 需要先保证唯一索引)
	// 这里使用 INSERT OR REPLACE 可能导致 id 重新分配 (sqlite 行替换)。
	// 更安全方式: 先尝试查询 id, 决定 INSERT 或 UPDATE。
	ex, err := r.GetByIPMI(m.IPMIIP)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if ex.ID == 0 { // insert
		// 加密存储
		encKey, _ := secret.EncryptString(m.SSHKey)
		res, err := r.db.Exec(`INSERT INTO machines (ipmi_ip, ssh_ip, ssh_user, ssh_key, remark) VALUES (?,?,?,?,?)`, m.IPMIIP, m.SSHIP, m.SSHUser, encKey, m.Remark)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		m.ID = int(id)
	} else { // update
		encKey, _ := secret.EncryptString(m.SSHKey)
		_, err := r.db.Exec(`UPDATE machines SET ssh_ip=?, ssh_user=?, ssh_key=?, remark=? WHERE ipmi_ip=?`, m.SSHIP, m.SSHUser, encKey, m.Remark, m.IPMIIP)
		if err != nil {
			return err
		}
		m.ID = ex.ID
	}
	return nil
}

// ListAll 返回全部机器（用于导出）。
func (r *MachineRepo) ListAll() ([]domain.Machine, error) {
	rows, err := r.db.Query(`SELECT id, ipmi_ip, ssh_ip, ssh_user, COALESCE(ssh_key,''), COALESCE(remark,''), COALESCE(created_at,'') FROM machines ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Machine
	for rows.Next() {
		var m domain.Machine
		var createdAtStr string
		if err := rows.Scan(&m.ID, &m.IPMIIP, &m.SSHIP, &m.SSHUser, &m.SSHKey, &m.Remark, &createdAtStr); err != nil {
			return nil, err
		}
		if createdAtStr != "" {
			if ts, e := time.Parse(time.RFC3339Nano, createdAtStr); e == nil {
				m.CreatedAt = ts
			}
		}
		if m.SSHKey != "" {
			if p, e := secret.DecryptString(m.SSHKey); e == nil && p != "" {
				m.SSHKey = p
			}
		}
		list = append(list, m)
	}
	return list, nil
}

// BulkUpsert 批量插入/更新（以 ipmi_ip 作为唯一键）。
// 若条目很多，使用事务一次性提交。
func (r *MachineRepo) BulkUpsert(ms []domain.Machine) error {
	if len(ms) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for i := range ms {
		m := &ms[i]
		// 直接复用 Save 逻辑，但使用 tx
		var exID int
		row := tx.QueryRow(`SELECT id FROM machines WHERE ipmi_ip = ? LIMIT 1`, m.IPMIIP)
		_ = row.Scan(&exID)
		encKey, _ := secret.EncryptString(m.SSHKey)
		if exID == 0 { // insert
			res, e := tx.Exec(`INSERT INTO machines (ipmi_ip, ssh_ip, ssh_user, ssh_key, remark) VALUES (?,?,?,?,?)`, m.IPMIIP, m.SSHIP, m.SSHUser, encKey, m.Remark)
			if e != nil {
				err = e
				return err
			}
			id, _ := res.LastInsertId()
			m.ID = int(id)
		} else { // update
			if _, e := tx.Exec(`UPDATE machines SET ssh_ip=?, ssh_user=?, ssh_key=?, remark=? WHERE ipmi_ip=?`, m.SSHIP, m.SSHUser, encKey, m.Remark, m.IPMIIP); e != nil {
				err = e
				return err
			}
			m.ID = exID
		}
	}
	return tx.Commit()
}

// DeleteByIPMI 根据 ipmi_ip 删除机器
func (r *MachineRepo) DeleteByIPMI(ip string) error {
	if strings.TrimSpace(ip) == "" {
		return errors.New("empty ip")
	}
	_, err := r.db.Exec(`DELETE FROM machines WHERE ipmi_ip=?`, ip)
	return err
}
