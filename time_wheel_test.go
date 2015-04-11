package turbo

import (
	"testing"
	"time"
)

func TestTimeWheel(t *testing.T) {
	tw := NewTimeWheel(100*time.Millisecond, 10, 100)
	ch := make(chan bool, 1)

	for i := 0; i < 10; i++ {
		to := false
		tw.After(1*time.Second, func() {
			ch <- true
		})
		select {
		case to = <-ch:
			to = true
		case <-time.After(2 * time.Second):
			to = false

		}
		if !to {
			t.Fail()
		} else {
			t.Logf("TestTimeWheel|SUCC|%d\n", i)
		}
	}
}
