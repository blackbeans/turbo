package turbo

import (
	"sync/atomic"
)

//network stat
type NetworkStat struct {
	ReadCount    int32 `json:"read_count"`
	ReadBytes    int32 `json:"read_bytes"`
	WriteCount   int32 `json:"write_count"`
	WriteBytes   int32 `json:"write_bytes"`
	DispatcherGo int32 `json:"dispatcher_go"`
	Connections  int32 `json:"connections"`
}

type RemotingFlow struct {
	Name           string
	OptimzeStatus  bool //当前优化的状态
	ReadFlow       *Flow
	ReadBytesFlow  *Flow
	DispatcherGo   int32
	WriteFlow      *Flow
	WriteBytesFlow *Flow
}

func NewRemotingFlow(name string) *RemotingFlow {
	return &RemotingFlow{
		OptimzeStatus:  true,
		Name:           name,
		ReadFlow:       &Flow{},
		ReadBytesFlow:  &Flow{},
		DispatcherGo:   0,
		WriteFlow:      &Flow{},
		WriteBytesFlow: &Flow{}}
}

//网络状态
func (self *RemotingFlow) Stat() NetworkStat {
	return NetworkStat{
		ReadCount:    self.ReadFlow.Changes(),
		ReadBytes:    self.ReadBytesFlow.Changes(),
		DispatcherGo: 0,
		WriteCount:   self.WriteFlow.Changes(),
		WriteBytes:   self.WriteBytesFlow.Changes(),
		Connections:  0}
}

type Flow struct {
	count     int64
	lastcount int64
}

func (self *Flow) Incr(num int32) {
	atomic.AddInt64(&self.count, int64(num))
}

func (self *Flow) Count() int32 {
	return int32(self.count)
}

func (self *Flow) Changes() int32 {
	tmpc := self.count
	tmpl := self.lastcount
	c := tmpc - tmpl
	self.lastcount = tmpc
	return int32(c)
}
