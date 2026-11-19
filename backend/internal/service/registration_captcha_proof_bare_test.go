package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/locallimit"
	"github.com/stretchr/testify/require"
)

type memoryRegistrationProofCache struct {
	mu       sync.Mutex
	tokens   map[string]*RegistrationCaptchaProofPayload
	restores int
}

func newMemoryRegistrationProofCache() *memoryRegistrationProofCache {
	return &memoryRegistrationProofCache{tokens: make(map[string]*RegistrationCaptchaProofPayload)}
}

func (c *memoryRegistrationProofCache) SetRegistrationCaptchaProof(_ context.Context, token string, payload *RegistrationCaptchaProofPayload, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[token] = payload
	return nil
}

func (c *memoryRegistrationProofCache) PeekRegistrationCaptchaProof(_ context.Context, token string) (*RegistrationCaptchaProofPayload, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens[token], nil
}

func (c *memoryRegistrationProofCache) TakeRegistrationCaptchaProof(_ context.Context, token string) (*RegistrationCaptchaProofPayload, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload := c.tokens[token]
	delete(c.tokens, token)
	return payload, nil
}

func (c *memoryRegistrationProofCache) RestoreRegistrationCaptchaProof(ctx context.Context, token string, payload *RegistrationCaptchaProofPayload, ttl time.Duration) error {
	c.mu.Lock()
	c.restores++
	c.mu.Unlock()
	return c.SetRegistrationCaptchaProof(ctx, token, payload, ttl)
}

func TestRegistrationCaptchaProofIssueValidateConsumeAndRestore(t *testing.T) {
	cache := newMemoryRegistrationProofCache()
	svc := NewRegistrationCaptchaProofService(cache)
	ctx := context.Background()

	token, err := svc.Issue(ctx, "User@Example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.NoError(t, err)
	require.Len(t, token, 48)

	_, err = svc.Validate(ctx, token, "user@example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.NoError(t, err)

	_, err = svc.Validate(ctx, token, "other@example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.True(t, errors.Is(err, ErrRegistrationCaptchaProofInvalid))

	payload, err := svc.Consume(ctx, token, "user@example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.NoError(t, err)
	require.NotNil(t, payload)

	_, err = svc.Consume(ctx, token, "user@example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.True(t, errors.Is(err, ErrRegistrationCaptchaProofInvalid))

	require.NoError(t, svc.Restore(ctx, token, payload))
	_, err = svc.Validate(ctx, token, "user@example.com", "203.0.113.10", "test-ua", "fingerprint")
	require.NoError(t, err)
}

func TestValidateRegistrationDeviceSignalsRejectsUAMismatchAndRandomization(t *testing.T) {
	svc := &AuthService{}
	ctx := context.Background()
	ua := "Mozilla/5.0 test"
	correct := &DeviceFingerprintSignals{
		UA:     HashDeviceFingerprintSignal(ua),
		TZ:     "tz",
		Lang:   "lang",
		Screen: "screen",
		Canvas: "canvas",
	}

	require.NoError(t, svc.ValidateRegistrationDeviceSignals(ctx, nil, ua, "203.0.113.10"))

	wrong := *correct
	wrong.UA = "not-a-valid-ua-hash"
	require.ErrorIs(t, svc.ValidateRegistrationDeviceSignals(ctx, &wrong, ua, "203.0.113.10"), ErrRegistrationDeviceSignalMismatch)

	for i := 0; i < deviceSignalEntropyThreshold; i++ {
		signals := *correct
		signals.Canvas = string(rune('a' + i))
		err := svc.ValidateRegistrationDeviceSignals(ctx, &signals, ua, "203.0.113.10")
		if i < deviceSignalEntropyThreshold-1 {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, ErrRegistrationDeviceSignalEntropy)
		}
	}
}

func TestLocalRateLimiterFallback(t *testing.T) {
	limiter := locallimit.New(8)
	now := time.Now()
	require.True(t, limiter.Allow("key", 2, time.Minute, now))
	require.True(t, limiter.Allow("key", 2, time.Minute, now))
	require.False(t, limiter.Allow("key", 2, time.Minute, now))
	require.True(t, limiter.Allow("key", 2, time.Minute, now.Add(time.Minute+time.Second)))
}
