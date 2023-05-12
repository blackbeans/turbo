package turbo

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

//利用原生channel做pool的管理
//example 1.0:
// 	tw:= schedule.NewTimeWheel(...)
// 	gpool := NewLimitPool(ctx,1000)
//  ctx,cancel := context.WithTimeOut(ctx,5* time.Second)
// 	wu,err := gpool.Queue(ctx,func(ctx context.Context)(interface{},error){
//   	return "hello",nil
// 	})
// cancel()
// 	if nil !=err {
//  	// 任务提交失败
// 		return
// 	}
// 	resp,err := wu.Get()
//
// example 2.0:
// 	tw:= schedule.NewTimeWheel(...)
// 	gpool := NewLimitPool(ctx,1000)
// ctx,cancel := context.WithTimeOut(ctx,5* time.Second)
// batch := gpool.NewBatch()
// 	wus,err := batch.Queue(func(ctx context.Context)(interface{},error){
//   	return "hello",nil
// 	}).Queue(func(ctx context.Context)(interface{},error){
//  	return "hello",nil
// 	}).Wait(ctx)
// cancel()
// 	if nil !=err {
//  	// 批量提交任务失败
// 		return
// 	}
//
//  获取结果
// for _,wu:=range wus{
// 	resp,err := wu.Get()
// }
//

//工作单元
type WorkUnit struct {
	ctx   context.Context
	once  sync.Once
	ch    chan *interface{} //future
	work  WorkFunc
	Value interface{}
	Err   error
}

func (self *WorkUnit) Get() (interface{}, error) {

	//要么结果已经有了， 要么你上下文结束了
	select {
	case <-self.ch:
		return self.Value, self.Err
		//获取到结果
	case <-self.ctx.Done():
		//上下文结束
		return self.Value, self.Err
	}
}

func (self *WorkUnit) AttachValue(val interface{}) {
	self.once.Do(func() {
		self.Value = val
		close(self.ch)
	})
}

//
func (self *WorkUnit) Error(err error) {
	self.once.Do(func() {
		self.Err = err
		close(self.ch)
	})
}

//处理函数
type WorkFunc func(ctx context.Context) (interface{}, error)

//goroutine pool
type GPool struct {
	maxcapcity int //最大容量
	limiter    chan interface{}
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewLimitPool(ctx context.Context, maxcapacity int) *GPool {
	limiter := make(chan interface{}, maxcapacity)
	ctx, cancel := context.WithCancel(ctx)
	return &GPool{
		ctx:     ctx,
		cancel:  cancel,
		limiter: limiter,
	}
}

var ERR_QUEUE_TIMEOUT = errors.New("Queue TIMEOUT!")

//取消
var ERR_QUEUE_CONTEXT_DONE = errors.New("Context is Done!")

//如果想使用超时获取的
func (self *GPool) Queue(ctx context.Context, work WorkFunc) (*WorkUnit, error) {
	wu := &WorkUnit{ch: make(chan *interface{}, 1), work: work, ctx: ctx}
	return wu, self.queue(wu)
}

//执行异步任务
func (self *GPool) queue(wu *WorkUnit) error {
	select {
	case <-wu.ctx.Done():
		wu.Error(ERR_QUEUE_CONTEXT_DONE)
		return wu.Err
	case self.limiter <- nil:
		go func() {
			defer func() {
				<-self.limiter
				if err := recover(); nil != err {
					log.Errorf("GPool|Queue|Panic|%v|%s", err, string(debug.Stack()))
					wu.Error(fmt.Errorf("%v", err))
				}
			}()

			select {
			case <-self.ctx.Done():
				wu.Error(ERR_QUEUE_CONTEXT_DONE)
				return
			case <-wu.ctx.Done():
				//当前工作单元上下文取消那么久取消
				wu.Error(ERR_QUEUE_CONTEXT_DONE)
				return
			default:
				//没有结束执行下面的
			}
			//执行异步方法
			val, err := wu.work(wu.ctx)
			if nil != err {
				wu.Error(err)
			} else {
				wu.AttachValue(val)
			}
		}()
	case <-self.ctx.Done():
		wu.Error(ERR_QUEUE_CONTEXT_DONE)
		return wu.Err
	}
	return nil
}

//返回当前正在运行goroutine、gopool的大小
func (self *GPool) Monitor() (int, int) {
	return len(self.limiter), cap(self.limiter)
}

func (self *GPool) Close() {
	self.cancel()
}

type Batch struct {
	gopool *GPool
	works  []WorkFunc
}

//创建一个批量任务
func (self *GPool) NewBatch() *Batch {
	return &Batch{
		gopool: self,
		works:  make([]WorkFunc, 0, 5),
	}
}

//batch增加QUEUE
func (self *Batch) Queue(work WorkFunc) *Batch {
	self.works = append(self.works, work)
	return self
}

//等待响应
//如果想使用超时结束等待的机制请使用 context.WithTimeOut()
func (self *Batch) Wait(ctx context.Context) ([]*WorkUnit, error) {

	wus := make([]*WorkUnit, 0, len(self.works))
	//不要重复关闭这个超时channel
	for i := range self.works {
		wu := &WorkUnit{
			ctx:  ctx,
			work: self.works[i],
			ch:   make(chan *interface{}, 1),
		}
		//提交异步处理
		err := self.gopool.queue(wu)
		if nil != err {
			if err == ERR_QUEUE_TIMEOUT {

			}
		}
		wus = append(wus, wu)
	}

	for i := range wus {
		select {
		case <-self.gopool.ctx.Done():
			//上层已经结束了
			for j := i; j < len(wus); j++ {
				//直接给结果
				if nil == wus[j].Err {
					wus[j].Error(ERR_QUEUE_CONTEXT_DONE)
				}
			}
			return wus, nil
		case <-wus[i].ctx.Done():
			//上层已经取消了
			for j := i; j < len(wus); j++ {
				//直接给结果
				if nil == wus[j].Err {
					wus[j].Error(ERR_QUEUE_CONTEXT_DONE)
				}
			}
			return wus, nil
		case <-wus[i].ch:
		}
	}
	return wus, nil
}
