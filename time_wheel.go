package turbo

import (
	"container/list"
	"sync"
	"time"
)

type slotJob struct {
	do  func()
	ttl int
}

type Slot struct {
	index int
	hooks *list.List
}

type TimeWheel struct {
	tick           *time.Ticker
	wheel          []*Slot
	ticksPerwheel  int
	tickPeriod     time.Duration
	currentTick    int
	slotJobWorkers chan bool
	lock           sync.RWMutex
}

//超时时间及每个个timewheel所需要的tick数
func NewTimeWheel(tickPeriod time.Duration, ticksPerwheel int, slotJobWorkers int) *TimeWheel {
	tw := &TimeWheel{
		tickPeriod:     tickPeriod,
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
			remove.PushFront(e)
			self.slotJobWorkers <- true
			//async
			go func() {
				defer func() {
					<-self.slotJobWorkers
				}()
				sj.do()
			}()
		}
	}
	self.lock.RUnlock()

	if nil != remove {
		//remove
		for e := remove.Back(); nil != e; e = e.Prev() {
			self.lock.Lock()
			slots.hooks.Remove(e)
			self.lock.Unlock()
		}
	}

}

//add timeout func
func (self *TimeWheel) After(timeout time.Duration, do func()) {

	self.lock.RLock()
	idx := self.preTickIndex()
	self.lock.RUnlock()

	self.lock.Lock()
	slots := self.wheel[idx]
	ttl := int(int64(timeout) / (int64(self.tickPeriod) * int64(self.ticksPerwheel)))
	job := &slotJob{do, ttl}
	slots.hooks.PushFront(job)
	self.lock.Unlock()
}

func (self *TimeWheel) preTickIndex() int {

	idx := self.currentTick
	if idx > 0 {
		idx -= 1
	} else {
		idx = self.ticksPerwheel - 1
	}

	return idx
}
