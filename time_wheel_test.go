package turbo

import (
	"testing"
	"time"
)

func TestTimeWheel(t *testing.T) {
	tw := NewTimeWheel(100*time.Millisecond, 10, 100)

	for i := 0; i < 10; i++ {
		to := false
		ch := make(chan bool, 1)
		tw.After(3*time.Second, func() {
			ch <- true
		})
		select {
		case to = <-ch:
			to = true
		case <-time.After(5 * time.Second):
			to = false

		}
		if !to {
			t.Fail()
		} else {
			t.Logf("TestTimeWheel|SUCC|%d\n", i)
		}
	}
}
