package handler

import (
	"context"
	"sync"
	"time"

	"github.com/tkalexx/shorturl.git/internal/logger"
	"go.uber.org/zap"
)

const (
	deleteWorkers    = 4
	deleteBatchSize  = 10
	deleteFlushEvery = 200 * time.Millisecond
)

type deleteTask struct {
	shortID string
	userID  string
}

// fanIn объединяет несколько каналов в один
func fanIn(channels ...<-chan deleteTask) <-chan deleteTask {
	var wg sync.WaitGroup
	out := make(chan deleteTask)

	multiplex := func(c <-chan deleteTask) {
		defer wg.Done()
		for task := range c {
			out <- task
		}
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go multiplex(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func (s *Service) startDeleteWorker() {
	go s.deleteWorker()
}

// DeleteUserURLs ставит идентификаторы URL в очередь на асинхронное удаление
func (s *Service) DeleteUserURLs(userID string, ids []string) {
	if len(ids) == 0 {
		return
	}

	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return
	}

	go s.enqueueDeletes(userID, filtered)
}

func (s *Service) enqueueDeletes(userID string, ids []string) {
	workers := deleteWorkers
	if workers > len(ids) {
		workers = len(ids)
	}

	channels := make([]<-chan deleteTask, 0, workers)
	chunkSize := (len(ids) + workers - 1) / workers

	for i := 0; i < workers; i++ {
		start := i * chunkSize
		if start >= len(ids) {
			break
		}
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		ch := make(chan deleteTask)
		go func(ids []string) {
			defer close(ch)
			for _, id := range ids {
				select {
				case ch <- deleteTask{shortID: id, userID: userID}:
				case <-s.done:
					return
				}
			}
		}(chunk)
		channels = append(channels, ch)
	}

	for task := range fanIn(channels...) {
		select {
		case s.deleteCh <- task:
		case <-s.done:
			return
		}
	}
}

func (s *Service) deleteWorker() {
	ticker := time.NewTicker(deleteFlushEvery)
	defer ticker.Stop()

	batch := make([]deleteTask, 0, deleteBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		byUser := make(map[string][]string)
		for _, task := range batch {
			byUser[task.userID] = append(byUser[task.userID], task.shortID)
		}

		for userID, ids := range byUser {
			if err := s.repo.MarkDeleted(context.Background(), ids, userID); err != nil {
				logger.Log.Error("failed to mark urls deleted", zap.Error(err), zap.String("user_id", userID))
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case task, ok := <-s.deleteCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, task)
			if len(batch) >= deleteBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.done:
			flush()
			return
		}
	}
}
