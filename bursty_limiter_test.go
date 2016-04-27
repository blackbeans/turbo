package turbo

import (
	"fmt"
	"testing"
	"time"
)

func TestBurstyLimiter(t *testing.T) {
	//
	limiter, _ := NewBurstyLimiter(5, 10)

	tw := NewTimeWheel(1*time.Microsecond, 50, 10)
	ch := make(chan bool, 1)
	for i := 0; i < 30; i++ {
		_, ch = tw.After(1*time.Millisecond, func() {})
		succ := limiter.TryAcquire(ch)
		if i < 5 && !succ {
			t.Fail()
		}
		fmt.Printf("PRE %d-------%v-----------%v\n", i, time.Now(), succ)
	}
	limiter.Destroy()

}

func TestBurstyLimiterTw(t *testing.T) {
	tw := NewTimeWheel(10*time.Microsecond, 10, 10)
	limiter, _ := NewBurstyLimiterWithTikcer(0, 10000, tw)

	ch := make(chan bool, 1)
	start := time.Now()
	// timeout := NewTimeWheel(1*time.Microsecond, 50, 10)
	for i := 0; i < 100000; i++ {
		// id, ch := timeout.After(20*time.Millisecond, func() {})

		succ := limiter.TryAcquire(ch)
		if !succ {
			t.Fail()
		}
		// timeout.Remove(id)
		// fmt.Printf("PRE %d-------%v-----------%v\n", i, time.Now(), succ)
	}

	fmt.Println((time.Now().UnixNano() - start.UnixNano()))
	limiter.Destroy()

}

func TestBurstyLimiter2(t *testing.T) {
	limiter, _ := NewBurstyLimiter(0, 1000)

	for i := 0; i < 30; i++ {
		ch := make(chan bool, 1)
		succ := limiter.TryAcquire(ch)
		if i < 5 && !succ {
			t.Fail()
		}
		if succ {
			fmt.Printf("PRE %d-------%v-----------%v\n", i, time.Now(), succ)
		}
	}
	limiter.Destroy()
}
