package turbo

import (
	"container/list"
	"fmt"
	log "github.com/blackbeans/log4go"
	"sync"
	"sync/atomic"
	"time"
)

var channelPool = &sync.Pool{New: func() interface{} {
	return make(chan bool, 1)
}}

type slotJob struct {
	id  int64
	do  func()
	ttl int
	ch  chan bool
}

type Slot struct {
	index int
	hooks *list.List
	sync.RWMutex
}

type TimeWheel struct {
	autoId         int64
	tick           *time.Ticker
	wheel          []*Slot
	hashWheel      map[int64]*list.Element
	ticksPerwheel  int
	tickPeriod     time.Duration
	currentTick    int32
	slotJobWorkers chan bool
	lock           sync.RWMutex
}

//超时时间及每个个timewheel所需要的tick数
func NewTimeWheel(tickPeriod time.Duration, ticksPerwheel int, slotJobWorkers int) *TimeWheel {
	tw := &TimeWheel{
		lock:           sync.RWMutex{},
		tickPeriod:     tickPeriod,
		hashWheel:      make(map[int64]*list.Element, 10000),
		tick:           time.NewTicker(tickPeriod),
		slotJobWorkers: make(chan bool, slotJobWorkers),
		wheel: func() []*Slot {
			//ticksPerWheel make ticksPerWheel+1 slide
			w := make([]*Slot, 0, ticksPerwheel+1)
			for i := 0; i < ticksPerwheel+1; i++ {
				w = append(w, func() *Slot {
					return &Slot{
						index: i,
						hooks: list.New()}
				}())
			}
			return w
		}(),
		ticksPerwheel: ticksPerwheel + 1,
		currentTick:   0}
	go func() {
		for i := 0; ; i++ {
			i = i % tw.ticksPerwheel
			<-tw.tick.C
			atomic.StoreInt32(&tw.currentTick, int32(i))
			tw.notifyExpired(i)
		}
	}()

	return tw
}

func (self *TimeWheel) Monitor() string {
	ticks := 0
	for _, v := range self.wheel {
		v.RLock()
		ticks += v.hooks.Len()
		v.RUnlock()
	}
	return fmt.Sprintf("TimeWheel|[total-tick:%d\tworkers:%d/%d]",
		ticks, len(self.slotJobWorkers), cap(self.slotJobWorkers))
}

//notifyExpired func
func (self *TimeWheel) notifyExpired(idx int) {
	var remove []*list.Element
	slots := self.wheel[idx]
	//-------clear expired
	slots.RLock()
	for e := slots.hooks.Back(); nil != e; e = e.Prev() {
		sj := e.Value.(*slotJob)
		sj.ttl--
		//ttl expired
		if sj.ttl <= 0 {
			if nil == remove {
				remove = make([]*list.Element, 0, 10)
			}

			//记录删除
			remove = append(remove, e)
			//写出超时
			sj.ch <- true

			self.slotJobWorkers <- true
			//async
			go func() {
				defer func() {
					if err := recover(); nil != err {
						//ignored
						log.Error("TimeWheel|notifyExpired|Do|ERROR|%s\n", err)
					}
					<-self.slotJobWorkers

				}()

				sj.do()

				// log.Debug("TimeWheel|notifyExpired|%d\n", sj.ttl)

			}()
		}
	}
	slots.RUnlock()

	if len(remove) > 0 {
		slots.Lock()
		//remove
		for _, v := range remove {
			slots.hooks.Remove(v)
		}
		slots.Unlock()

		self.lock.Lock()
		//remove
		for _, v := range remove {
			job := v.Value.(*slotJob)
			delete(self.hashWheel, job.id)
		}
		self.lock.Unlock()
	}

}

//add timeout func
func (self *TimeWheel) After(timeout time.Duration, do func()) (int64, chan bool) {

	idx := self.preTickIndex()
	ttl := int(int64(timeout) / (int64(self.tickPeriod) * int64(self.ticksPerwheel)))
	// log.Debug("After|TTL:%d|%d\n", ttl, timeout)
	id := self.timerId(idx)
	ch := channelPool.Get().(chan bool)
	job := &slotJob{id, do, ttl, ch}

	slots := self.wheel[idx]
	slots.Lock()
	e := slots.hooks.PushFront(job)
	slots.Unlock()

	self.lock.Lock()
	self.hashWheel[id] = e
	self.lock.Unlock()
	return id, job.ch
}

func (self *TimeWheel) Remove(timerId int64) {

	sid := self.decodeSlot(timerId)
	sl := self.wheel[sid]
	self.lock.Lock()
	e, ok := self.hashWheel[timerId]
	self.lock.Unlock()
	if ok {
		sl.Lock()
		job := sl.hooks.Remove(e)
		sl.Unlock()
		channelPool.Put(job.(*slotJob).ch)

	}

}

func (self *TimeWheel) decodeSlot(timerId int64) int {
	return int(timerId >> 32)
}

func (self *TimeWheel) timerId(idx int32) int64 {

	return int64(int64(idx<<32) | (atomic.AddInt64(&self.autoId, 1) >> 32))
}

func (self *TimeWheel) preTickIndex() int32 {
	idx := atomic.LoadInt32(&self.currentTick)
	if idx > 0 {
		idx -= 1
	} else {
		idx = int32(self.ticksPerwheel - 1)
	}
	return idx
}
