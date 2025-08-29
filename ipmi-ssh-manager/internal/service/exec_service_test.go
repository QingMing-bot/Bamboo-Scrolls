package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/repository"
	sshmock "github.com/QingMing-Bot/ipmi-ssh-manager/internal/ssh"

	_ "modernc.org/sqlite"
)

// helper 打开内存库并建表
func openMemDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE machines( id INTEGER PRIMARY KEY AUTOINCREMENT, ipmi_ip TEXT UNIQUE, ssh_ip TEXT, ssh_user TEXT, ssh_key TEXT, remark TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, zbx_id TEXT );`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE exec_history( id INTEGER PRIMARY KEY AUTOINCREMENT, machine_id INTEGER, ipmi_ip TEXT, command TEXT, stdout TEXT, stderr TEXT, exit_code INTEGER, error_text TEXT, started_at TIMESTAMP, finished_at TIMESTAMP, duration_ms INTEGER );`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestExecService_BatchExec(t *testing.T) {
	db := openMemDB(t)
	defer db.Close()
	repo := repository.NewMachineRepo(db)
	m1 := domain.Machine{IPMIIP: "10.0.0.1", SSHIP: "10.0.0.1", SSHUser: "root"}
	m2 := domain.Machine{IPMIIP: "10.0.0.2", SSHIP: "10.0.0.2", SSHUser: "root"}
	if err := repo.Save(&m1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(&m2); err != nil {
		t.Fatal(err)
	}

	hRepo := repository.NewHistoryRepo(db)
	hWriter := NewHistoryWriter(hRepo, 1, 10)
	defer hWriter.Close()

	mock := sshmock.NewMockExecutor()
	mock.Set("hostname", sshmock.MockResult{Stdout: "node1\n", ExitCode: 0})

	svc := NewExecService(repo, hWriter, mock, 2)
	res, err := svc.BatchExec(domain.ExecTask{Command: "hostname", Timeout: 5, MachineIDs: []int64{int64(m1.ID), int64(m2.ID)}})
	if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expect 2 results got %d", len(res))
	}

	// 等待异步写入 flush
	time.Sleep(1500 * time.Millisecond)
	rows, err := hRepo.ListRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("history not written")
	}
}

// Test job start & cancel using a slow mock command
func TestExecService_JobCancel(t *testing.T) {
	db := openMemDB(t)
	defer db.Close()
	repo := repository.NewMachineRepo(db)
	m1 := domain.Machine{IPMIIP: "10.0.0.10", SSHIP: "10.0.0.10", SSHUser: "root"}
	if err := repo.Save(&m1); err != nil {
		t.Fatal(err)
	}
	hRepo := repository.NewHistoryRepo(db)
	hWriter := NewHistoryWriter(hRepo, 1, 10)
	defer hWriter.Close()
	mock := sshmock.NewMockExecutor()
	// Delay large to allow cancel
	mock.Set("sleep", sshmock.MockResult{Stdout: "done\n", ExitCode: 0, DelayMs: 3000})
	svc := NewExecService(repo, hWriter, mock, 1)
	// context not required; StartBatch internally manages its own context
	jobID, err := svc.StartBatch("", domain.ExecTask{Command: "sleep", Timeout: 5, MachineIDs: []int64{int64(m1.ID)}}, func(r domain.ExecResult) {})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	// Cancel quickly
	time.Sleep(200 * time.Millisecond)
	ok := svc.Cancel(jobID)
	if !ok {
		t.Fatalf("cancel returned false")
	}
	// Wait until job map cleared
	deadline := time.Now().Add(2 * time.Second)
	for svc.HasJob(jobID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if svc.HasJob(jobID) {
		t.Fatalf("job still present after cancel")
	}
}

// Test history cleanup logic via HistoryRepo.Cleanup
func TestHistoryRepo_Cleanup(t *testing.T) {
	db := openMemDB(t)
	defer db.Close()
	hRepo := repository.NewHistoryRepo(db)
	// create table already in openMemDB
	// insert 5 rows with staggered dates
	now := time.Now()
	for i := 0; i < 5; i++ {
		h := domain.ExecHistory{MachineID: 1, IPMIIP: "1.1.1.1", Command: "cmd", StartedAt: now.Add(-time.Duration(i) * 24 * time.Hour), FinishedAt: now.Add(-time.Duration(i) * 24 * time.Hour)}
		if err := hRepo.Insert(&h); err != nil {
			t.Fatal(err)
		}
	}
	// keep only last 2 days
	if err := hRepo.Cleanup(2, 0); err != nil {
		t.Fatalf("cleanup err: %v", err)
	}
	rows, err := hRepo.ListRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if now.Sub(r.StartedAt) > 48*time.Hour {
			t.Fatalf("row older than retention remains: %v", r.StartedAt)
		}
	}
	// now enforce max rows 1
	if err := hRepo.Cleanup(0, 1); err != nil {
		t.Fatalf("cleanup rows err: %v", err)
	}
	rows, err = hRepo.ListRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row remained, got %d", len(rows))
	}
}
