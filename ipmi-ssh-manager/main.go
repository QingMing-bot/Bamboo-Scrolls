package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/repository"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/service"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/ssh"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/wailsapi"
	"github.com/QingMing-Bot/ipmi-ssh-manager/pkg/config"
	"github.com/QingMing-Bot/ipmi-ssh-manager/webui"
)

func main() {
	cfg := config.Load()
	db, err := sql.Open("sqlite", cfg.DBPath())
	if err != nil {
		log.Fatal(err)
	}
	repo := repository.NewMachineRepo(db)
	_ = repo.EnsureSchema()
	hRepo := repository.NewHistoryRepo(db)
	hWriter := service.NewHistoryWriter(hRepo, cfg.HistoryFlushInterval, cfg.HistoryBatchSize)
	if cfg.HistoryRetentionDays > 0 || cfg.HistoryMaxRows > 0 {
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				_ = hRepo.Cleanup(cfg.HistoryRetentionDays, cfg.HistoryMaxRows)
			}
		}()
	}
	executor := ssh.NewExecutor(cfg.MaxParallel)
	execSvc := service.NewExecService(repo, hWriter, executor, cfg.MaxParallel)
	backend := wailsapi.NewBackend(db, repo, hRepo, execSvc)
	app := &options.App{
		Title:       "IPMI SSH Manager",
		Width:       1200,
		Height:      800,
		AssetServer: &assetserver.Options{Assets: webui.Assets},
		Bind:        []interface{}{backend},
		OnStartup: func(ctx context.Context) {
			backend.SetCtx(ctx)
			runtime.LogInfo(ctx, "Wails backend context initialized")
		},
	}
	if err := wails.Run(app); err != nil {
		log.Fatal(err)
	}
	hWriter.Close()
}
