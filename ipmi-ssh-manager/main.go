package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/remoteapi"
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
	// 根据配置决定本地还是远程仓库
	var (
		mRepo     repository.MachineRepoIface
		hRepo     repository.HistoryRepoIface
		useRemote = cfg.RemoteAPIBase != ""
	)
	if useRemote {
		remoteClient, err := remoteapi.New(cfg.RemoteAPIBase, func() string { return cfg.RemoteAPIToken })
		if err != nil {
			log.Fatalf("remote client init failed: %v", err)
		}
		mRepo = remoteapi.NewRemoteMachineRepo(remoteClient)
		hRepo = remoteapi.NewRemoteHistoryRepo(remoteClient)
	} else {
		localM := repository.NewMachineRepo(db)
		_ = localM.EnsureSchema()
		localH := repository.NewHistoryRepo(db)
		_ = localH.EnsureSchema()
		mRepo = localM
		hRepo = localH
	}
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
	execSvc := service.NewExecService(mRepo, hWriter, executor, cfg.MaxParallel)
	backend := wailsapi.NewBackend(db, mRepo, hRepo, execSvc)
	// 设置全局 key provider，允许执行时回退使用 (机器未配置单独 key 时)
	execSvc.SetGlobalKeyProvider(func() string { return backend.GetGlobalSSHKey() })

	app := &options.App{
		Title:       "IPMI SSH Manager",
		Width:       1180,
		Height:      820,
		MinWidth:    960, // 允许缩小
		MinHeight:   600, // 允许缩小
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
