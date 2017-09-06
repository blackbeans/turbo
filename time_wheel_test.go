package turbo

import "testing"
import "time"

var tw *TimerWheel = NewTimerWheel(200*time.Millisecond, 10)

func TestTimeWheel(t *testing.T) {

	id, ch := tw.After(5 * time.Second)
	start := time.Now().Unix()
	select {
	case <-ch:
		t.Logf("5 Seconds Timeout !")
	case <-time.After(6 * time.Second):
		tw.CancelTimer(id)
		t.FailNow()
		t.Logf("TestTimeWheel 5 Seconds Not Timeout ")
	}

	t.Logf("Wait : %d s", time.Now().Unix()-start)

	//多线程添加超时
	chs := make([]chan time.Time, 0, 10)
	for i := 0; i < 10; i++ {
		_, ch := tw.After(5 * time.Second)
		chs = append(chs, ch)
	}

	for _, ch := range chs {
		start = time.Now().Unix()
		select {
		case <-ch:
			t.Logf("5 Seconds Timeout !")
		case <-time.After(6 * time.Second):
			tw.CancelTimer(id)
			t.FailNow()
			t.Logf("TestTimeWheel 5 Seconds Not Timeout ")
		}
		t.Logf("Wait : %d s", time.Now().Unix()-start)
	}

	//超时取消
	id, ch = tw.After(5 * time.Second)
	tw.CancelTimer(id)

	start = time.Now().Unix()
	select {
	case <-ch:
		t.Logf("5 Seconds Timeout !")
		t.FailNow()
	case <-time.After(7 * time.Second):
		t.Logf("TestTimeWheel 7 Seconds Not Timeout ")
	}
	t.Logf("Cancel Succ : %d s", time.Now().Unix()-start)

	//update timeout
	id, ch = tw.After(5 * time.Second)
	time.Sleep(2 * time.Second)
	tw.UpdateTimer(id, time.Now().Add(10*time.Second))
	start = time.Now().Unix()
	select {
	case <-ch:
		t.Logf(" Seconds Timeout ! %d", time.Now().Unix()-start)
		t.FailNow()
	case <-time.After(5 * time.Second):
		t.Logf("TestTimeWheel 5 Seconds  Timeout Should 12s")
	}

	//add timer
	//update timeout

	id, ch = tw.AddTimer(5*time.Second, func(now time.Time) {
		t.Logf("Timeout %v\n", now)
	}, func(t time.Time) {

	})

	select {
	case <-ch:
		t.Logf("TestTimeWheel 5 Seconds  Timeout  ")
	case <-time.After(6 * time.Second):
		t.FailNow()
		t.Logf("TestTimeWheel 5 Seconds  Timeout Should 12s")
	}
}