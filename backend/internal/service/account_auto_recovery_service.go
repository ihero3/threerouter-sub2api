package service

import (
	"context"
	"log"
	"sync"
	"time"
)

// AccountAutoRecoveryService 周期性恢复因 failover 429 配额错误被自动禁用、
// 且已到达记录的配额重置时间的账号（status: error → active）。
//
// 只恢复 error_message 带 FailoverAutoRecoverMessageTag 标记的账号，
// 人工禁用或认证失败等其他 error 账号不受影响。
type AccountAutoRecoveryService struct {
	accountRepo AccountRepository
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

func NewAccountAutoRecoveryService(accountRepo AccountRepository, interval time.Duration) *AccountAutoRecoveryService {
	return &AccountAutoRecoveryService{
		accountRepo: accountRepo,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (s *AccountAutoRecoveryService) Start() {
	if s == nil || s.accountRepo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AccountAutoRecoveryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *AccountAutoRecoveryService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updated, err := s.accountRepo.AutoRecoverFailoverDisabledAccounts(ctx, time.Now())
	if err != nil {
		log.Printf("[AccountAutoRecovery] Auto recover failover disabled accounts failed: %v", err)
		return
	}
	if updated > 0 {
		log.Printf("[AccountAutoRecovery] Auto recovered %d failover disabled accounts", updated)
	}
}
