package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
)

type HistoryRepo struct{ db *sql.DB }

func NewHistoryRepo(db *sql.DB) *HistoryRepo { return &HistoryRepo{db: db} }

func (r *HistoryRepo) Insert(h *domain.ExecHistory) error {
	now := time.Now()
	if h.StartedAt.IsZero() {
		h.StartedAt = now
	}
	if h.FinishedAt.IsZero() {
		h.FinishedAt = now
	}
	res, err := r.db.Exec(`INSERT INTO exec_history(machine_id,ipmi_ip,command,stdout,stderr,exit_code,error_text,started_at,finished_at,duration_ms)
        VALUES (?,?,?,?,?,?,?,?,?,?)`, h.MachineID, h.IPMIIP, h.Command, h.Stdout, h.Stderr, h.ExitCode, h.ErrorText, h.StartedAt, h.FinishedAt, h.DurationMs)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	h.ID = id
	return nil
}

func (r *HistoryRepo) ListRecent(limit int) ([]domain.ExecHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`SELECT id,machine_id,ipmi_ip,command,stdout,stderr,exit_code,error_text,started_at,finished_at,duration_ms FROM exec_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.ExecHistory
	for rows.Next() {
		var h domain.ExecHistory
		if err := rows.Scan(&h.ID, &h.MachineID, &h.IPMIIP, &h.Command, &h.Stdout, &h.Stderr, &h.ExitCode, &h.ErrorText, &h.StartedAt, &h.FinishedAt, &h.DurationMs); err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, nil
}

// ListFiltered 支持按 ipmi_ip 与 command 关键字过滤 (模糊匹配)。传空表示忽略该条件。
func (r *HistoryRepo) ListFiltered(limit int, ipmi, cmdLike string) ([]domain.ExecHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	where := ""
	args := []any{}
	if ipmi != "" {
		where += " AND ipmi_ip LIKE ?"
		args = append(args, "%"+ipmi+"%")
	}
	if cmdLike != "" {
		where += " AND command LIKE ?"
		args = append(args, "%"+cmdLike+"%")
	}
	q := `SELECT id,machine_id,ipmi_ip,command,stdout,stderr,exit_code,error_text,started_at,finished_at,duration_ms FROM exec_history WHERE 1=1` + where + ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.ExecHistory
	for rows.Next() {
		var h domain.ExecHistory
		if err := rows.Scan(&h.ID, &h.MachineID, &h.IPMIIP, &h.Command, &h.Stdout, &h.Stderr, &h.ExitCode, &h.ErrorText, &h.StartedAt, &h.FinishedAt, &h.DurationMs); err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, nil
}

// Cleanup 根据保留天数与最大行数裁剪
func (r *HistoryRepo) Cleanup(retentionDays, maxRows int) error {
	if retentionDays > 0 {
		_, _ = r.db.Exec(`DELETE FROM exec_history WHERE started_at < datetime('now', ?)`, fmt.Sprintf("-%d days", retentionDays))
	}
	if maxRows > 0 {
		// 删除超过 maxRows 的最旧行
		_, _ = r.db.Exec(`DELETE FROM exec_history WHERE id IN (SELECT id FROM exec_history ORDER BY id DESC LIMIT -1 OFFSET ?)`, maxRows)
	}
	return nil
}
