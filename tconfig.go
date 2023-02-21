package turbo

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	CONCURRENT_LEVEL = 8
)

//-----------响应的future
type Future struct {
	timeout time.Duration
	opaque  uint32
	once    sync.Once
	ch      chan interface{}

	response   interface{}
	TargetHost string
	Err        error
	ctx        context.Context
}

func NewFuture(opaque uint32, timeout time.Duration, targetHost string, ctx context.Context) *Future {

	return &Future{
		timeout:    timeout,
		opaque:     opaque,
		ch:         make(chan interface{}, 1),
		TargetHost: targetHost,
		ctx:        ctx,
		Err:        nil}
}

//创建有错误的future
func NewErrFuture(opaque uint32, targetHost string, err error, ctx context.Context) *Future {
	f := &Future{
		timeout:    0,
		opaque:     opaque,
		ch:         make(chan interface{}, 1),
		TargetHost: targetHost,
		ctx:        ctx}
	f.Error(err)
	return f
}

func (f *Future) Error(err error) {
	f.once.Do(func() {
		f.Err = err
		close(f.ch)
	})

}

func (f *Future) SetResponse(resp interface{}) {
	f.once.Do(func() {
		f.response = resp
		close(f.ch)
	})
}

func (f *Future) Get(timeout <-chan time.Time) (interface{}, error) {

	select {
	case <-timeout:
		//如果是已经超时了但是当前还是没有响应也认为超时
		f.Error(ERR_TIMEOUT)
		return f.response, f.Err
	case <-f.ctx.Done():
		f.Error(ERR_CONNECTION_BROKEN)
		return f.response, f.Err
	case <-f.ch:
		return f.response, f.Err
	}
}

//网络层参数
type TConfig struct {
	FlowStat         *RemotingFlow //网络层流量
	dispool          *GPool        //   最大分发处理协程数
	ReadBufferSize   int           //读取缓冲大小
	WriteBufferSize  int           //写入缓冲大小
	WriteChannelSize int           //写异步channel长度
	ReadChannelSize  int           //读异步channel长度
	IdleTime         time.Duration //连接空闲时间
	RequestHolder    *ReqHolder
	TW               *TimerWheel // timewheel
	cancel           context.CancelFunc
}

func NewTConfig(name string,
	maxdispatcherNum,
	readbuffersize,
	writebuffersize,
	writechannlesize,
	readchannelsize int,
	idletime time.Duration,
	maxOpaque int) *TConfig {

	tw := NewTimerWheel(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	rh := &ReqHolder{
		opaque:   0,
		holder:   NewLRUCache(ctx, maxOpaque, tw, nil),
		tw:       tw,
		idleTime: idletime}

	dispool := NewLimitPool(ctx, maxdispatcherNum)
	//初始化
	rc := &TConfig{
		FlowStat:         NewRemotingFlow(name, dispool),
		dispool:          dispool,
		ReadBufferSize:   readbuffersize,
		WriteBufferSize:  writebuffersize,
		WriteChannelSize: writechannlesize,
		ReadChannelSize:  readchannelsize,
		IdleTime:         idletime,
		RequestHolder:    rh,
		TW:               tw,
		cancel:           cancel,
	}
	return rc
}

type ReqHolder struct {
	opaque   uint32
	tw       *TimerWheel
	holder   *LRUCache
	idleTime time.Duration //连接空闲时间
}

func (self *ReqHolder) CurrentOpaque() uint32 {
	return uint32(atomic.AddUint32(&self.opaque, 1))
}

//从requesthold中移除
func (self *ReqHolder) Detach(opaque uint32, obj interface{}) {

	future := self.holder.Remove(opaque)
	if nil != future {
		future.(*Future).SetResponse(obj)
	}
}

func (self *ReqHolder) Attach(opaque uint32, future *Future) chan time.Time {
	return self.holder.Put(opaque, future, future.timeout)
}
