package handler

import (
	"context"
	"time"

	"github.com/tkalexx/shorturl.git/internal/logger"
	"go.uber.org/zap"
)

const (
	deleteBatchSize  = 10
	deleteFlushEvery = 200 * time.Millisecond
)

type deleteTask struct {
	shortID string
	userID  string
}

func (s *Service) startDeleteWorker() {
	s.wg.Add(1)
	go s.deleteWorker()
}

// Close останавливает воркер удаления и сбрасывает накопленный батч в хранилище.
func (s *Service) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	s.wg.Wait()
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

	go func() {
		for _, id := range filtered {
			select {
			case s.deleteCh <- deleteTask{shortID: id, userID: userID}:
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Service) deleteWorker() {
	defer s.wg.Done()

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
			for {
				select {
				case task := <-s.deleteCh:
					batch = append(batch, task)
					if len(batch) >= deleteBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}
