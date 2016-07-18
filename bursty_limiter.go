package turbo

import (
	"golang.org/x/time/rate"
	"time"
)

type BurstyLimiter struct {
	rateLimiter *rate.Limiter
}

func NewBurstyLimiter(initPermits int, permitsPerSecond int) (*BurstyLimiter, error) {

	limiter := rate.NewLimiter(rate.Limit(permitsPerSecond), initPermits)

	return &BurstyLimiter{rateLimiter: limiter}, nil
}

func (self *BurstyLimiter) PermitsPerSecond() int {
	return int(self.rateLimiter.Limit())
}

//try acquire token
func (self *BurstyLimiter) AcquireCount(count int) bool {
	return self.rateLimiter.AllowN(time.Now(), count)
}

//acquire token
func (self *BurstyLimiter) Acquire() bool {
	return self.rateLimiter.Allow()
}

//return 1 : acquired
//return 2 : total
func (self *BurstyLimiter) LimiterInfo() (int, int) {
	return int(self.rateLimiter.Limit()) - self.rateLimiter.Burst(), int(self.rateLimiter.Limit())
}

func (self *BurstyLimiter) Destroy() {

}
