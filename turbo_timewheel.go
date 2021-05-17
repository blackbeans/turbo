package turbo

import (
	"container/heap"
	"sync/atomic"
	"time"
)

type OnEvent func(t time.Time)

//一个timer任务
type Timer struct {
	InitTid  uint32 //初始化tid
	timerId  uint32
	Index    int
	expired  time.Time
	interval time.Duration
	//回调函数
	onTimeout OnEvent
	onCancel  OnEvent
	//是否重复过期
	repeated bool
}

type TimerHeap []*Timer

func (h TimerHeap) Len() int { return len(h) }

func (h TimerHeap) Less(i, j int) bool {
	if h[i].expired.Before(h[j].expired) {
		return true
	}

	if h[i].expired.After(h[j].expired) {
		return false
	}
	return h[i].timerId < h[j].timerId
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

const (
	MIN_INTERVAL = 100 * time.Millisecond
)

func timerId() uint32 {
	return atomic.AddUint32(&timerIds, 1)
}

//时间轮
type TimerWheel struct {
	timerHeap   TimerHeap
	tick        *time.Ticker
	hashTimer   map[uint32]*Timer
	interval    time.Duration
	cancelTimer chan uint32
	addTimer    chan *Timer
	updateTimer chan Timer
	workLimit   chan *interface{}
}

func NewTimerWheel(interval time.Duration, workSize int) *TimerWheel {

	if int64(interval)-int64(MIN_INTERVAL) < 0 {
		interval = MIN_INTERVAL
	}

	tw := &TimerWheel{
		timerHeap:   make(TimerHeap, 0),
		tick:        time.NewTicker(interval),
		hashTimer:   make(map[uint32]*Timer, 10),
		interval:    interval,
		updateTimer: make(chan Timer, 2000),
		cancelTimer: make(chan uint32, 2000),
		addTimer:    make(chan *Timer, 2000),
		workLimit:   make(chan *interface{}, workSize*2),
	}
	heap.Init(&tw.timerHeap)
	tw.start()
	return tw
}

//timerwheel的状态
//add :=> 添加定时器的队列长度
//update:=>更新时间轮的队列长度
//cancel:=>取消时间轮的队列长度
//worker:=>超时、取消时处理队列长度
func (self *TimerWheel) Monitor() (add, update, cancel, worker int) {
	return len(self.addTimer), len(self.updateTimer), len(self.cancelTimer), len(self.workLimit)
}

func (self *TimerWheel) After(timeout time.Duration) (uint32, chan time.Time) {
	if timeout < self.interval {
		timeout = self.interval
	}
	ch := make(chan time.Time, 1)
	tid := timerId()
	t := &Timer{
		timerId:   tid,
		InitTid:   tid,
		expired:   time.Now().Add(timeout),
		onTimeout: func(t time.Time) { ch <- t },
		onCancel:  nil}

	self.addTimer <- t
	return t.timerId, ch
}

//周期性的timer
//返回初始的timerid
func (self *TimerWheel) RepeatedTimer(interval time.Duration,
	onTimout OnEvent, onCancel OnEvent) uint32 {
	if interval < self.interval {
		interval = self.interval
	}
	tid := timerId()
	t := &Timer{
		repeated: true,
		interval: interval,
		timerId:  tid,
		InitTid:  tid,
		expired:  time.Now().Add(interval),
		onTimeout: func(t time.Time) {
			if nil != onTimout {
				onTimout(t)
			}
		},
		onCancel: onCancel}

	self.addTimer <- t
	return tid
}

func (self *TimerWheel) AddTimer(timeout time.Duration, onTimout OnEvent, onCancel OnEvent) (uint32, chan time.Time) {
	ch := make(chan time.Time, 1)
	tid := timerId()
	t := &Timer{
		timerId:  tid,
		InitTid:  tid,
		interval: timeout,
		expired:  time.Now().Add(timeout),
		onTimeout: func(t time.Time) {
			defer func() {
				ch <- t
			}()
			if nil != onTimout {
				onTimout(t)
			}
		},
		onCancel: onCancel}

	self.addTimer <- t
	return t.timerId, ch
}

//更新timer的时间
func (self *TimerWheel) UpdateTimer(timerid uint32, expired time.Time) {
	t := Timer{
		timerId: timerid,
		expired: expired}
	self.updateTimer <- t
}

//取消一个id
func (self *TimerWheel) CancelTimer(timerid uint32) {
	self.cancelTimer <- timerid
}

//同步操作
func (self *TimerWheel) checkExpired(now time.Time) {
	for {
		if self.timerHeap.Len() <= 0 {
			break
		}

		expired := self.timerHeap[0].expired
		//如果过期时间再当前tick之前则超时
		//或者当前时间和过期时间的差距在一个Interval周期内那么就认为过期的
		if expired.After(now) {
			break
		}
		t := heap.Pop(&self.timerHeap).(*Timer)
		if nil != t.onTimeout {
			//这里极有可能出现处理任务线程池开的太小
			//导致整个时间轮等待，添加不了，造成死锁
			self.workLimit <- nil
			go func() {
				defer func() {
					<-self.workLimit
				}()
				t.onTimeout(now)
			}()

			//如果是repeated的那么就检查并且重置过期时间
			if t.repeated {
				//如果是需要repeat的那么继续放回去
				t.expired = t.expired.Add(t.interval)
				if !t.expired.After(now) {
					t.expired = now.Add(t.interval)
				}
				//重新加入这个repeated 时间
				//但是初始timerid不会变
				t.timerId = timerId()
				self.onAddTimer(t)
			} else {
				delete(self.hashTimer, t.timerId)
				delete(self.hashTimer, t.InitTid)
			}
		} else {
			delete(self.hashTimer, t.timerId)
			delete(self.hashTimer, t.InitTid)
		}

	}
}

//
func (self *TimerWheel) start() {

	go func() {
		for {
			select {
			case now := <-self.tick.C:
				//这里极有可能出现等待，超时任务可能处理耗时
				self.checkExpired(now)
			case updateT := <-self.updateTimer:
				if t, ok := self.hashTimer[updateT.timerId]; ok {
					t.expired = updateT.expired
					heap.Fix(&self.timerHeap, t.Index)
				}

			case t := <-self.addTimer:
				self.onAddTimer(t)
			case timerid := <-self.cancelTimer:
				if t, ok := self.hashTimer[timerid]; ok {
					delete(self.hashTimer, t.InitTid)
					delete(self.hashTimer, t.timerId)
					heap.Remove(&self.timerHeap, t.Index)
					if nil != t.onCancel {
						self.workLimit <- nil
						go func() {
							<-self.workLimit
							t.onCancel(time.Now())
						}()
					}
				}
			}
		}
	}()
}

func (self *TimerWheel) onAddTimer(t *Timer) {
	heap.Push(&self.timerHeap, t)
	//只有不是repeated那么加入hash结构，做对应关系
	if !t.repeated {
		self.hashTimer[t.timerId] = t
	} else {
		//注意这里只记录repeated的初始的timerid,用于后续取消定时任务
		self.hashTimer[t.InitTid] = t
	}
}
