package turbo

import (
	"container/heap"
	"sync/atomic"
	"time"
)

type OnEvent func(tid uint32, t time.Time)

//一个timer任务
type Timer struct {
	InitTid uint32 //初始化tid
	timerId uint32
	Index   int
	ttl     time.Duration
	//回调函数
	onTimeout OnEvent
	onCancel  OnEvent
	//是否重复过期
	repeated bool
	interval time.Duration //周期
}

type TimerHeap []*Timer

func (h TimerHeap) Len() int { return len(h) }

func (h TimerHeap) Less(i, j int) bool {
	if h[i].ttl == h[j].ttl {
		return h[i].timerId < h[j].timerId
	}

	return h[i].ttl < h[j].ttl
}

func (h TimerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}

func (h *TimerHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*Timer)
	item.Index = n
	*h = append(*h, item)
}

func (h *TimerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	item.Index = -1 // for safety
	*h = old[0 : n-1]
	return item
}

var timerIds uint32 = 0

type IntervalRange struct {
	min      time.Duration
	max      time.Duration
	interval time.Duration
}

var intervalRanges = []IntervalRange{
	{min: 0, max: 100 * time.Millisecond, interval: 5 * time.Millisecond},
	{min: 100 * time.Millisecond, max: 500 * time.Millisecond, interval: 10 * time.Millisecond},
	{min: 500 * time.Millisecond, max: time.Second, interval: 50 * time.Millisecond},
	{min: time.Second, max: 10 * time.Second, interval: 100 * time.Millisecond},
	{min: 10 * time.Second, max: time.Minute, interval: 500 * time.Millisecond},
	{min: time.Minute, max: time.Hour, interval: time.Second},
}

func timerId() uint32 {
	return atomic.AddUint32(&timerIds, 1)
}

type timeoutEvent struct {
	tid uint32
	now time.Time
	do  OnEvent
}

//时间轮
type TimerWheel struct {
	createTs    time.Time //时间轮创建时间
	timerHeap   TimerHeap
	tick        *time.Ticker
	hashTimer   map[uint32]*Timer
	interval    time.Duration
	cancelTimer chan uint32
	addTimer    chan *Timer
	updateTimer chan Timer

	//定期更新milsecond
	currMils int64

	//全局处理超时的timer
	onTimerCh       chan timeoutEvent
	extraTimeOutGos int64 //记录当前额外的goroutines处理超时任务无的goroutines
}

func NewTimerWheel(interval time.Duration) *TimerWheel {

	matched := false
	for _, irange := range intervalRanges {
		if interval >= irange.min && interval < irange.max {
			interval = irange.interval
			matched = true
			break
		}
	}

	if !matched {
		//没有命中，那么选取最大的周期
		interval = intervalRanges[len(intervalRanges)-1].interval
	}

	tw := &TimerWheel{
		createTs:    time.Now(),
		timerHeap:   make(TimerHeap, 0),
		tick:        time.NewTicker(interval),
		hashTimer:   make(map[uint32]*Timer, 10),
		interval:    interval,
		updateTimer: make(chan Timer, 2000),
		cancelTimer: make(chan uint32, 2000),
		addTimer:    make(chan *Timer, 2000),
		onTimerCh:   make(chan timeoutEvent, 1000), //超时
	}
	heap.Init(&tw.timerHeap)
	tw.start()
	return tw
}

//timerwheel的状态
//add :=> 添加定时器的队列长度
//update:=>更新时间轮的队列长度
//cancel:=>取消时间轮的队列长度
//timeout:=>处理任务超时时候的队列长度
//超时任务独立携程处理个数
func (tw *TimerWheel) Monitor() (add, update, cancel int, timeout int, extraTimeOutGos int64) {
	return len(tw.addTimer), len(tw.updateTimer), len(tw.cancelTimer), len(tw.onTimerCh), tw.extraTimeOutGos
}

func (tw *TimerWheel) After(timeout time.Duration) (uint32, chan time.Time) {
	if timeout < tw.interval {
		timeout = tw.interval
	}
	ch := make(chan time.Time, 1)
	tid := timerId()
	t := &Timer{
		timerId:   tid,
		InitTid:   tid,
		ttl:       tw.now().Add(timeout).Sub(tw.createTs),
		onTimeout: func(tid uint32, t time.Time) { ch <- t },
		onCancel:  nil}

	tw.addTimer <- t
	return t.timerId, ch
}

//周期性的timer
//返回初始的timerid
func (tw *TimerWheel) RepeatedTimer(interval time.Duration,
	onTimout OnEvent, onCancel OnEvent) uint32 {
	if interval < tw.interval {
		interval = tw.interval
	}
	tid := timerId()
	t := &Timer{
		repeated: true,
		timerId:  tid,
		InitTid:  tid,
		interval: interval,
		ttl:      tw.now().Add(interval).Sub(tw.createTs),
		onTimeout: func(tid uint32, t time.Time) {
			if nil != onTimout {
				onTimout(tid, t)
			}
		},
		onCancel: onCancel}

	tw.addTimer <- t
	return tid
}

func (tw *TimerWheel) now() time.Time {
	mils := atomic.LoadInt64(&tw.currMils)
	return time.Unix(mils/1000, mils%1000*int64(time.Millisecond))
}

func (tw *TimerWheel) CurrentMilSeconds() int64 {
	return atomic.LoadInt64(&tw.currMils)
}

func (tw *TimerWheel) AddTimer(timeout time.Duration, onTimout OnEvent, onCancel OnEvent) (uint32, chan time.Time) {
	ch := make(chan time.Time, 1)
	tid := timerId()
	t := &Timer{
		timerId: tid,
		InitTid: tid,
		ttl:     tw.now().Add(timeout).Sub(tw.createTs),
		onTimeout: func(tid uint32, t time.Time) {
			defer func() {
				ch <- t
			}()
			if nil != onTimout {
				onTimout(tid, t)
			}
		},
		onCancel: onCancel}

	tw.addTimer <- t
	return t.timerId, ch
}

//更新timer的时间
func (tw *TimerWheel) UpdateTimer(timerid uint32, expired time.Time) {
	t := Timer{
		timerId: timerid,
		ttl:     expired.Sub(tw.createTs),
	}
	tw.updateTimer <- t
}

//取消一个id
func (tw *TimerWheel) CancelTimer(timerid uint32) {
	tw.cancelTimer <- timerid
}

//同步操作
func (tw *TimerWheel) checkExpired(now time.Time, expiredTTL time.Duration) {
	for {
		if tw.timerHeap.Len() <= 0 {
			break
		}

		//如果过期时间再当前tick之前则超时
		//或者当前时间和过期时间的差距在一个Interval周期内那么就认为过期的
		if tw.timerHeap[0].ttl > expiredTTL {
			break
		}
		t := heap.Pop(&tw.timerHeap).(*Timer)
		if nil != t.onTimeout {
			//调用超时处理任务
			select {
			case tw.onTimerCh <- timeoutEvent{tid: t.InitTid, now: now, do: t.onTimeout}:
			default:
				//如果onTimerCh 满了，那么久主动降级创建独立协程序处理
				atomic.AddInt64(&tw.extraTimeOutGos, 1)
				go func() {
					defer func() {
						atomic.AddInt64(&tw.extraTimeOutGos, -1)
					}()
					t.onTimeout(t.InitTid, now)
				}()
			}

			//如果是repeated的那么就检查并且重置过期时间
			if t.repeated {
				//如果是需要repeat的那么继续放回去
				t.ttl = expiredTTL + t.interval
				//重新加入这个repeated 时间
				//但是初始timerid不会变
				t.timerId = timerId()
				tw.onAddTimer(t)
			} else {
				delete(tw.hashTimer, t.timerId)
				delete(tw.hashTimer, t.InitTid)
			}
		} else {
			delete(tw.hashTimer, t.timerId)
			delete(tw.hashTimer, t.InitTid)
		}

	}
}

//
func (tw *TimerWheel) start() {

	//启动超时任务的消费携程
	go func() {
		for {
			select {
			case event := <-tw.onTimerCh:
				//如果有超时的直接回掉处理
				event.do(event.tid, event.now)
			}
		}
	}()

	atomic.StoreInt64(&tw.currMils, time.Now().UnixNano()/int64(time.Millisecond))
	go func() {
		for {
			atomic.StoreInt64(&tw.currMils, time.Now().UnixNano()/int64(time.Millisecond))
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		elapse := time.Duration(0)
		preT := time.Now()
		for {
			select {
			case t := <-tw.tick.C:
				now := tw.now()
				//前一个时间是小的说明正常
				if t.Sub(preT) >= time.Millisecond {
					//逝去的时间加上偏移的
					elapse += t.Sub(preT)
				} else {
					//说明时间调调小了
					//看下启动时间纠正为ttl个
					elapse = elapse + tw.interval
					tw.createTs = now.Add(-elapse)
				}
				preT = t
				//这里极有可能出现等待，超时任务可能处理耗时
				tw.checkExpired(now, elapse)
			case updateT := <-tw.updateTimer:
				tw.onUpdate(updateT)
				tw.onFastConsume()
			case t := <-tw.addTimer:
				tw.onAddTimer(t)
				//让添加更快消化了
				tw.onFastConsume()
			case tid := <-tw.cancelTimer:
				//让取消更快消化了
				tw.onCancel(tid)
				tw.onFastConsume()
			}
		}
	}()
}

func (tw *TimerWheel) onFastConsume() {
outter:
	for {
		select {
		case tmpT := <-tw.addTimer:
			tw.onAddTimer(tmpT)
		case tmpT := <-tw.updateTimer:
			tw.onUpdate(tmpT)
		case tmpT := <-tw.cancelTimer:
			tw.onCancel(tmpT)
		default:
			break outter
		}
	}
}

func (tw *TimerWheel) onUpdate(updateT Timer) {

	if t, ok := tw.hashTimer[updateT.timerId]; ok {
		t.ttl = updateT.ttl
		heap.Fix(&tw.timerHeap, t.Index)
	}
}

func (tw *TimerWheel) onCancel(tid uint32) {
	if t, ok := tw.hashTimer[tid]; ok {
		if t.repeated {
			delete(tw.hashTimer, t.InitTid)
		}
		delete(tw.hashTimer, t.timerId)
		heap.Remove(&tw.timerHeap, t.Index)
		if nil != t.onCancel {
			go t.onCancel(t.InitTid, tw.now())
		}
	}
}

func (tw *TimerWheel) onAddTimer(t *Timer) {
	heap.Push(&tw.timerHeap, t)
	//只有不是repeated那么加入hash结构，做对应关系
	if !t.repeated {
		tw.hashTimer[t.timerId] = t
	} else {
		//注意这里只记录repeated的初始的timerid,用于后续取消定时任务
		tw.hashTimer[t.InitTid] = t
	}
}
