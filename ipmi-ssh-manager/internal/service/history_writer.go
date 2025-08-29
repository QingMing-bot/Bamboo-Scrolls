package service

import (
	"sync"
	"time"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/repository"
)

type HistoryWriter struct {
	repo          *repository.HistoryRepo
	ch            chan domain.ExecHistory
	stop          chan struct{}
	flushInterval time.Duration
	batchSize     int
	wg            sync.WaitGroup
}

func NewHistoryWriter(repo *repository.HistoryRepo, flushSec int, batchSize int) *HistoryWriter {
	if flushSec <= 0 {
		flushSec = 2
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	hw := &HistoryWriter{repo: repo, ch: make(chan domain.ExecHistory, batchSize*4), stop: make(chan struct{}), flushInterval: time.Duration(flushSec) * time.Second, batchSize: batchSize}
	hw.wg.Add(1)
	go hw.loop()
	return hw
}

func (w *HistoryWriter) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	batch := make([]domain.ExecHistory, 0, w.batchSize)
	flush := func() {
		for i := range batch {
			h := batch[i]
			_ = w.repo.Insert(&h)
		}
		batch = batch[:0]
	}
	for {
		select {
		case h := <-w.ch:
			batch = append(batch, h)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flush()
			}
		case <-w.stop:
			if len(batch) > 0 {
				flush()
			}
			return
		}
	}
}

func (w *HistoryWriter) Write(h domain.ExecHistory) {
	select {
	case w.ch <- h:
	default: /* drop if full */
	}
}

func (w *HistoryWriter) Close() { close(w.stop); w.wg.Wait() }
