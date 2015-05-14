package turbo

import (
	"fmt"
	"sync/atomic"
)

//network stat
type NetworkStat struct {
	ReadCount       int32 `json:"read_count"`
	WriteCount      int32 `json:"write_count"`
	DispatcherCount int32 `json:"dispatcher_count"`
	DispatcherGo    int32 `json:"dispatcher_go"`
	ConnectionCount int32 `json:"connection_count"`
}

type RemotingFlow struct {
	Name               string
	OptimzeStatus      bool //当前优化的状态
	ReadFlow           *Flow
	DispatcherWorkPool *Flow //处理
	DispatcherFlow     *Flow
	WriteFlow          *Flow
	currentStat        *NetworkStat
}

func NewRemotingFlow(name string) *RemotingFlow {
	return &RemotingFlow{
		OptimzeStatus:      true,
		Name:               name,
		ReadFlow:           &Flow{},
		DispatcherWorkPool: &Flow{},
		DispatcherFlow:     &Flow{},
		WriteFlow:          &Flow{},
		currentStat:        &NetworkStat{}}
}

func (self *RemotingFlow) Monitor() string {

	self.currentStat.ReadCount = self.ReadFlow.Changes()
	self.currentStat.WriteCount = self.WriteFlow.Changes()
	self.currentStat.DispatcherCount = self.DispatcherFlow.Changes()

	line := fmt.Sprintf("%s:\t\tread:%d\t\tdispatcher:%d\t\twrite:%d\t\t", self.Name, self.currentStat.ReadCount,
		self.currentStat.DispatcherCount, self.currentStat.WriteCount)
	if nil != self.DispatcherWorkPool {
		self.currentStat.DispatcherGo = self.DispatcherWorkPool.count
		line = fmt.Sprintf("%sdispatcher-pool:%d\t\t", line, self.currentStat.DispatcherGo)
	}
	return line
}

//网络状态
func (self *RemotingFlow) Stat() NetworkStat {
	return *self.currentStat
}

type Flow struct {
	count     int32
	lastcount int32
}

func (self *Flow) Incr(num int32) {
	atomic.AddInt32(&self.count, num)
}

func (self *Flow) Count() int32 {
	return self.count
}

func (self *Flow) Changes() int32 {
	tmpc := self.count
	tmpl := self.lastcount
	c := tmpc - tmpl
	self.lastcount = tmpc
	return c
}
