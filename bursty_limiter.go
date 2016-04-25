package turbo

import (
	"errors"
	"fmt"
	"time"
)

type BurstyLimiter struct {
	permitsPerSecond int
	limiter          chan time.Time
	ticker           *time.Ticker
}

func NewBurstyLimiter(initPermits int, permitsPerSecond int) (*BurstyLimiter, error) {

	if initPermits < 0 || (permitsPerSecond < initPermits) {
		return nil,
			errors.New(fmt.Sprintf("BurstyLimiter initPermits[%d]<=permitsPerSecond[%d]", initPermits, permitsPerSecond))
	}
	pertick := int64(1*time.Second) / int64(permitsPerSecond)

	if pertick <= 0 {
		return nil,
			errors.New(fmt.Sprintf("BurstyLimiter int64(1*time.Second)< permitsPerSecond[%d]", permitsPerSecond))
	}

	tick := time.NewTicker(time.Duration(pertick))
	ch := make(chan time.Time, permitsPerSecond)
	for i := 0; i < initPermits; i++ {
		ch <- time.Now()
	}
	//insert token
	go func() {
		for t := range tick.C {
			ch <- t
		}
	}()

	return &BurstyLimiter{permitsPerSecond: permitsPerSecond, ticker: tick, limiter: ch}, nil
}

// Ticker (%d) [1s/permitsPerSecond] Must be greater than tw.tickPeriod
func NewBurstyLimiterWithTikcer(initPermits int, permitsPerSecond int, tw *TimeWheel) (*BurstyLimiter, error) {

	pertick := int64(1*time.Second) / int64(permitsPerSecond)

	if pertick < int64(tw.tickPeriod) {
		return nil, errors.New(fmt.Sprintf("Ticker (%d) [1s/permitsPerSecond] Must be greater than %d ", pertick, tw.tickPeriod))
	}

	ch := make(chan time.Time, permitsPerSecond)
	for i := 0; i < initPermits; i++ {
		ch <- time.Now()
	}

	//insert token
	go func() {
		for {
			//registry
			_, timeout := tw.After(time.Duration(pertick), func() {})

			<-timeout
			ch <- time.Now()
		}
	}()

	return &BurstyLimiter{permitsPerSecond: permitsPerSecond, limiter: ch}, nil
}

//try acquire token
func (self *BurstyLimiter) TryAcquire(timeout chan bool) bool {
	select {
	case <-self.limiter:
		return true
	case <-timeout:
		return false
	}
	return false
}

func (self *BurstyLimiter) Destroy() {
	if nil != self.ticker {
		self.ticker.Stop()
	}
}
