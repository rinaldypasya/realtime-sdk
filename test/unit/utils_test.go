package unit

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSnowflake(t *testing.T) {
	sf := utils.NewSnowflake(1)

	t.Run("GenerateUnique", func(t *testing.T) {
		ids := make(map[int64]bool)
		for i := 0; i < 10000; i++ {
			id := sf.Generate()
			assert.False(t, ids[id], "ID should be unique")
			ids[id] = true
		}
	})

	t.Run("GenerateOrdered", func(t *testing.T) {
		var lastID int64
		for i := 0; i < 1000; i++ {
			id := sf.Generate()
			assert.Greater(t, id, lastID, "IDs should be monotonically increasing")
			lastID = id
		}
	})

	t.Run("ConcurrentGeneration", func(t *testing.T) {
		ids := make(map[int64]bool)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					id := sf.Generate()
					mu.Lock()
					assert.False(t, ids[id], "ID should be unique even with concurrent generation")
					ids[id] = true
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
	})
}

func TestRandomString(t *testing.T) {
	t.Run("Length", func(t *testing.T) {
		for _, length := range []int{8, 16, 32, 64} {
			s := utils.RandomString(length)
			assert.Len(t, s, length)
		}
	})

	t.Run("Unique", func(t *testing.T) {
		strings := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			s := utils.RandomString(16)
			assert.False(t, strings[s], "Random strings should be unique")
			strings[s] = true
		}
	})
}

func TestRetry(t *testing.T) {
	t.Run("SuccessOnFirstAttempt", func(t *testing.T) {
		attempts := 0
		err := utils.Retry(func() error {
			attempts++
			return nil
		}, utils.RetryConfig{
			MaxAttempts: 3,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond * 10,
			Multiplier:  2,
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("SuccessAfterRetries", func(t *testing.T) {
		attempts := 0
		err := utils.Retry(func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}, utils.RetryConfig{
			MaxAttempts: 5,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond * 10,
			Multiplier:  2,
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("MaxAttemptsExceeded", func(t *testing.T) {
		attempts := 0
		expectedErr := errors.New("persistent error")
		err := utils.Retry(func() error {
			attempts++
			return expectedErr
		}, utils.RetryConfig{
			MaxAttempts: 3,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond * 10,
			Multiplier:  2,
		})

		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 3, attempts)
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("AllowWithinLimit", func(t *testing.T) {
		limiter := utils.NewRateLimiter(10, 10) // 10 tokens, 10/second refill

		for i := 0; i < 10; i++ {
			assert.True(t, limiter.Allow(), "Should allow requests within limit")
		}
	})

	t.Run("DenyOverLimit", func(t *testing.T) {
		limiter := utils.NewRateLimiter(5, 0) // 5 tokens, no refill

		for i := 0; i < 5; i++ {
			assert.True(t, limiter.Allow())
		}

		assert.False(t, limiter.Allow(), "Should deny requests over limit")
	})

	t.Run("RefillOverTime", func(t *testing.T) {
		limiter := utils.NewRateLimiter(1, 1000) // 1 token, 1000/second refill

		assert.True(t, limiter.Allow())
		assert.False(t, limiter.Allow())

		time.Sleep(time.Millisecond * 10) // Wait for refill

		assert.True(t, limiter.Allow())
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("ClosedState", func(t *testing.T) {
		cb := utils.NewCircuitBreaker(3, time.Second)

		assert.Equal(t, utils.CircuitClosed, cb.State())
		assert.True(t, cb.Allow())
	})

	t.Run("OpenAfterFailures", func(t *testing.T) {
		cb := utils.NewCircuitBreaker(3, time.Second)

		cb.Failure()
		cb.Failure()
		assert.True(t, cb.Allow()) // Still closed

		cb.Failure() // Third failure
		assert.Equal(t, utils.CircuitOpen, cb.State())
		assert.False(t, cb.Allow())
	})

	t.Run("ResetOnSuccess", func(t *testing.T) {
		cb := utils.NewCircuitBreaker(3, time.Second)

		cb.Failure()
		cb.Failure()
		cb.Success()

		assert.Equal(t, utils.CircuitClosed, cb.State())
		cb.Failure()
		cb.Failure()
		cb.Failure()
		// Would need 3 more failures to open
		assert.Equal(t, utils.CircuitOpen, cb.State())
	})

	t.Run("HalfOpenAfterTimeout", func(t *testing.T) {
		cb := utils.NewCircuitBreaker(3, time.Millisecond*50)

		// Open the circuit
		cb.Failure()
		cb.Failure()
		cb.Failure()
		assert.Equal(t, utils.CircuitOpen, cb.State())

		// Wait for reset timeout
		time.Sleep(time.Millisecond * 100)

		// Should transition to half-open
		assert.True(t, cb.Allow())
		assert.Equal(t, utils.CircuitHalfOpen, cb.State())
	})
}

func TestDefaultRetryConfig(t *testing.T) {
	config := utils.DefaultRetryConfig()

	assert.Equal(t, 5, config.MaxAttempts)
	assert.Equal(t, time.Second, config.InitialWait)
	assert.Equal(t, time.Minute, config.MaxWait)
	assert.Equal(t, 2.0, config.Multiplier)
}
