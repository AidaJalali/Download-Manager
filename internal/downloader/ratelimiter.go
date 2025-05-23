package downloader

import (
	"io"
	"sync"
	"time"
)

type RateLimiter struct {
	tokensPerSecond int64
	bucketSize      int64
	currentTokens   int64
	lastRefill      time.Time
	mutex           sync.Mutex
	tokenChan       chan struct{}
	stopChan        chan struct{}
}

func NewRateLimiter(bytesPerSecond int64) *RateLimiter {
	bucketSize := bytesPerSecond
	limiter := &RateLimiter{
		tokensPerSecond: bytesPerSecond,
		bucketSize:      bucketSize,
		currentTokens:   bucketSize,
		lastRefill:      time.Now(),
		tokenChan:       make(chan struct{}, 1000),
		stopChan:        make(chan struct{}),
	}
	go limiter.generateTokens()
	return limiter
}

func (r *RateLimiter) generateTokens() {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.refillTokens()
		}
	}
}

func (r *RateLimiter) refillTokens() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	newTokens := int64(float64(r.tokensPerSecond) * elapsed)
	if r.currentTokens+newTokens > r.bucketSize {
		newTokens = r.bucketSize - r.currentTokens
	}
	r.currentTokens += newTokens

	for r.currentTokens > 0 {
		select {
		case r.tokenChan <- struct{}{}:
			r.currentTokens--
		default:
			return
		}
	}
}

func (r *RateLimiter) GetToken(bytes int64) {
	tokensNeeded := bytes
	for tokensNeeded > 0 {
		<-r.tokenChan
		tokensNeeded--
	}
}

// Read reads data from reader and applies rate limiting
// It implements a rate-limited reader for the download process
func (r *RateLimiter) Read(reader io.Reader, buffer []byte) (int, error) {
	// First read data from the original reader
	n, err := reader.Read(buffer)
	if n > 0 {
		// If data was read, limit the rate by getting tokens
		r.GetToken(int64(n))
	}
	return n, err
}

func (r *RateLimiter) Stop() {
	close(r.stopChan)
}
