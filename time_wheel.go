package turbo

import (
	"container/list"
	"fmt"
	log "github.com/blackbeans/log4go"
	"sync"
	"sync/atomic"
	"time"
)

type slotJob struct {
	id  int64
	do  func()
	ttl int
	ch  chan bool
}

type Slot struct {
	index int
	hooks *list.List
}

type TimeWheel struct {
	autoId         int64
	tick           *time.Ticker
	wheel          []*Slot
	hashWheel      map[int64]*list.Element
	ticksPerwheel  int
	tickPeriod     time.Duration
	currentTick    int
	slotJobWorkers chan bool
	lock           *sync.RWMutex
}

//超时时间及每个个timewheel所需要的tick数
func NewTimeWheel(tickPeriod time.Duration, ticksPerwheel int, slotJobWorkers int) *TimeWheel {
	tw := &TimeWheel{
		lock:           &sync.RWMutex{},
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
			tw.lock.Lock()
			tw.currentTick = i
			tw.lock.Unlock()
			//notify expired
			tw.notifyExpired(i)
		}
	}()

	return tw
}

func (self *TimeWheel) Monitor() string {
	ticks := 0
	for _, v := range self.wheel {
		ticks += v.hooks.Len()
	}
	return fmt.Sprintf("TimeWheel|[total-tick:%d\tworkers:%d/%d]",
		ticks, len(self.slotJobWorkers), cap(self.slotJobWorkers))
}

//notifyExpired func
func (self *TimeWheel) notifyExpired(idx int) {
	var remove *list.List
	self.lock.RLock()
	slots := self.wheel[idx]
	for e := slots.hooks.Back(); nil != e; e = e.Prev() {
		sj := e.Value.(*slotJob)
		sj.ttl--
		//ttl expired
		if sj.ttl <= 0 {
			if nil == remove {
				remove = list.New()
			}
			//记录删除
			remove.PushFront(e)

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

				sj.ch <- true
				close(sj.ch)
				// log.Debug("TimeWheel|notifyExpired|%d\n", sj.ttl)

			}()
		}
	}

	self.lock.RUnlock()

	if nil != remove {
		//remove
		for e := remove.Back(); nil != e; e = e.Prev() {
			re := e.Value.(*list.Element)
			func() {
				self.lock.Lock()
				defer self.lock.Unlock()
				slots.hooks.Remove(e.Value.(*list.Element))
				delete(self.hashWheel, re.Value.(*slotJob).id)

			}()
		}
	}

}

//add timeout func
func (self *TimeWheel) After(timeout time.Duration, do func()) (int64, chan bool) {

	idx := self.preTickIndex()

	self.lock.Lock()
	slots := self.wheel[idx]
	ttl := int(int64(timeout) / (int64(self.tickPeriod) * int64(self.ticksPerwheel)))
	// log.Debug("After|TTL:%d|%d\n", ttl, timeout)
	id := self.timerId(idx)
	job := &slotJob{id, do, ttl, make(chan bool, 1)}
	e := slots.hooks.PushFront(job)
	self.hashWheel[id] = e
	self.lock.Unlock()
	return id, job.ch
}

func (self *TimeWheel) Remove(timerId int64) {
	self.lock.Lock()
	e, ok := self.hashWheel[timerId]
	if ok {
		sid := self.decodeSlot(timerId)
		sl := self.wheel[sid]
		sl.hooks.Remove(e)
		delete(self.hashWheel, timerId)
	}
	self.lock.Unlock()
}

func (self *TimeWheel) decodeSlot(timerId int64) int {
	return int(timerId >> 32)
}

func (self *TimeWheel) timerId(idx int) int64 {

	return int64(int64(idx<<32) | (atomic.AddInt64(&self.autoId, 1) >> 32))
}

func (self *TimeWheel) preTickIndex() int {
	self.lock.RLock()
	idx := self.currentTick
	if idx > 0 {
		idx -= 1
	} else {
		idx = self.ticksPerwheel - 1
	}
	self.lock.RUnlock()
	return idx
}
