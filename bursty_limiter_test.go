package turbo

import (
	"testing"
	"time"
)

func TestBurstyLimiter(t *testing.T) {

	limiter, _ := NewBurstyLimiter(5, 10)

	for i := 0; i < 30; i++ {
		succ := limiter.Acquire()
		if i < 5 && !succ {
			t.Fail()
		} else if !succ {
			time.Sleep(100 * time.Millisecond)
			succ = limiter.Acquire()
			if !succ {
				t.Fail()
				return
			}
			t.Logf("TestBurstyLimiter|FAIL|RECOVER %d-------%v\n", i, succ)
		} else {
			t.Logf("TestBurstyLimiter %d-------%v\n", i, succ)
		}

	}
	limiter.Destroy()

}

func TestBurstyLimiter2(t *testing.T) {
	limiter, _ := NewBurstyLimiter(5, 1000)
	succ := limiter.AcquireCount(5)
	if !succ {
		t.Fail()
	}
	t.Logf("TestBurstyLimiter2|AcquireCount|5|-------%v\n", succ)
	for i := 0; i < 30; i++ {
		succ := limiter.Acquire()
		if i < 5 && succ {
			t.Fail()
		} else if i > 5 {
			time.Sleep(1 * time.Millisecond)
			succ = limiter.Acquire()
			t.Logf("TestBurstyLimiter2|LEFT 5 |SUCC| %d-------%v\n", i, succ)
		} else {
			t.Logf("TestBurstyLimiter2|FIRST 5 |FAIL| %d-------%v\n", i, succ)
		}
	}
	limiter.Destroy()
}
