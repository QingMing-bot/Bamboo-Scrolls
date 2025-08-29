package repository

import (
	"database/sql"
	"runtime"
	"testing"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/pkg/secret"
	_ "modernc.org/sqlite"
)

func openMemMachines(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE machines( id INTEGER PRIMARY KEY AUTOINCREMENT, ipmi_ip TEXT UNIQUE, ssh_ip TEXT, ssh_user TEXT, ssh_key TEXT, remark TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, zbx_id TEXT );`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMachineRepo_Save_EncryptsKey(t *testing.T) {
	db := openMemMachines(t)
	defer db.Close()
	repo := NewMachineRepo(db)
	m := domain.Machine{IPMIIP: "1.1.1.1", SSHUser: "root", SSHKey: "PRIVATE_KEY"}
	if err := repo.Save(&m); err != nil {
		t.Fatalf("save error: %v", err)
	}
	// 直接查询底层存储值
	var raw string
	if err := db.QueryRow(`SELECT ssh_key FROM machines WHERE ipmi_ip=?`, m.IPMIIP).Scan(&raw); err != nil {
		t.Fatalf("scan raw: %v", err)
	}
	if runtime.GOOS == "windows" {
		if raw == m.SSHKey {
			t.Fatalf("expected encrypted different from plain")
		}
		if len(raw) < len(secret.Prefix) || raw[:len(secret.Prefix)] != secret.Prefix {
			t.Fatalf("missing prefix in stored value: %q", raw)
		}
	} else {
		// 非 Windows 仍可能是明文
		if raw != m.SSHKey {
			t.Fatalf("expected passthrough plain key")
		}
	}
	// 读取 API 应返回解密后的明文（windows）或原文（非 windows）
	got, err := repo.GetByIPMI(m.IPMIIP)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got.SSHKey != "PRIVATE_KEY" {
		t.Fatalf("expected decrypted key got %q", got.SSHKey)
	}
}
