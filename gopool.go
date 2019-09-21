package turbo

import (
	"context"
	"errors"
	"fmt"
	"github.com/blackbeans/log4go"
	"runtime/debug"
	"time"
)

//利用原生channel做pool的管理
//example 1.0:
// 	tw:= schedule.NewTimeWheel(...)
// 	gpool := NewLimitPool(ctx,tw,1000)
// 	wu,err := gpool.Queue(func(ctx context.Context)(interface{},error){
//   	return "hello",nil
// 	},5 * time.Second)
// 	if nil !=err {
//  	// 任务提交失败
// 		return
// 	}
// 	resp,err := wu.Get()
//
// example 2.0:
// 	tw:= schedule.NewTimeWheel(...)
// 	gpool := NewLimitPool(ctx,tw,1000)
// batch := gpool.NewBatch()
// 	wus,err := batch.Queue(func(ctx context.Context)(interface{},error){
//   	return "hello",nil
// 	}).Queue(func(ctx context.Context)(interface{},error){
//  	return "hello",nil
// 	}).Wait(5 * time.Second)
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
	cancel context.CancelFunc //任务取消
	ch     chan *interface{}  //future
	Value  interface{}
	Err    error
}

func (self *WorkUnit) Get() (interface{}, error) {
	if nil != self.Err {
		return self.Value, self.Err
	}
	<-self.ch
	return self.Value, self.Err
}

//取消这个任务
func (self *WorkUnit) Cancel() {
	self.cancel()
}

//处理函数
type WorkFunc func(ctx context.Context) (interface{}, error)

//goroutine pool
type GPool struct {
	maxcapcity int //最大容量
	limiter    chan interface{}
	ctx        context.Context
	cancel     context.CancelFunc
	tw         *TimerWheel
}

func NewLimitPool(ctx context.Context, tw *TimerWheel, maxcapacity int) *GPool {
	limiter := make(chan interface{}, maxcapacity)
	ctx, cancel := context.WithCancel(ctx)
	return &GPool{
		ctx:     ctx,
		cancel:  cancel,
		limiter: limiter,
		tw:      tw,
	}
}

var ERR_QUEUE_TIMEOUT = errors.New("Queue TIMEOUT!")

//取消
var ERR_QUEUE_CANCEL = errors.New("Queue CANCEL!")

func (self *GPool) Queue(work WorkFunc, timeout time.Duration) (*WorkUnit, error) {
	if timeout > 0 {
		//提交阶段的超时
		tid, tch := self.tw.After(timeout)
		defer self.tw.CancelTimer(tid)
		return self.queue(work, tch)
	} else {
		return self.queue(work, nil)
	}
}

//执行异步任务
func (self *GPool) queue(work WorkFunc, timeout chan time.Time) (*WorkUnit, error) {

	wu := &WorkUnit{ch: make(chan *interface{}, 1)}
	ctx, cancel := context.WithCancel(self.ctx)
	wu.cancel = cancel
	if nil != timeout {
		select {
		case self.limiter <- nil:
			go func() {
				defer func() {
					<-self.limiter
					if err := recover(); nil != err {
						log4go.ErrorLog("handler", "GPool|Queue|Panic|%v|%s", err, string(debug.Stack()))
						wu.Err = fmt.Errorf("%v", err)
					}
					close(wu.ch)
					cancel()
				}()

				select {
				case <-ctx.Done():
					//当前工作单元上下文取消那么久取消
					wu.Err = ERR_QUEUE_CANCEL
					return
				default:
					//没有结束执行下面的
				}
				//执行异步方法
				wu.Value, wu.Err = work(ctx)
				if nil != wu.Err {
					log4go.ErrorLog("handler", "GPool|Queue|work|FAIL|%v", wu.Err)
				}
			}()
		case <-timeout:
			wu.Err = ERR_QUEUE_TIMEOUT
			close(wu.ch)
			cancel()
			return wu, wu.Err
		}
	} else {
		self.limiter <- nil
		//没有超时直接无限等待
		go func() {
			defer func() {
				<-self.limiter
				if err := recover(); nil != err {
					log4go.ErrorLog("handler", "GPool|Queue|Panic|%v|%s", err, string(debug.Stack()))
					wu.Err = fmt.Errorf("%v", err)
				}
				close(wu.ch)
				cancel()
			}()

			select {
			case <-ctx.Done():
				//当前工作单元上下文取消那么久取消
				wu.Err = ERR_QUEUE_CANCEL
				return
			default:
				//没有结束执行下面的
			}

			//执行异步方法
			wu.Value, wu.Err = work(ctx)
			if nil != wu.Err {
				log4go.ErrorLog("handler", "GPool|Queue|work|FAIL|%v", wu.Err)
			}
		}()
	}
	return wu, nil
}

//创建一个批量任务
func (self *GPool) NewBatch() *Batch {
	return &Batch{
		gopool: self,
		works:  make([]WorkFunc, 0, 5),
	}
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

//batch增加QUEUE
func (self *Batch) Queue(work WorkFunc) *Batch {
	self.works = append(self.works, work)
	return self
}

//等待响应
func (self *Batch) Wait(timeout time.Duration) ([]*WorkUnit, error) {

	wus := make([]*WorkUnit, 0, len(self.works))
	var timeoutCh chan time.Time
	if timeout > 0 {
		tid, tch := self.gopool.tw.After(timeout)
		defer self.gopool.tw.CancelTimer(tid)
		timeoutCh = tch
	}

	timeoutClosed := false
	for i := range self.works {
		work := self.works[i]
		//提交异步处理
		wu, err := self.gopool.queue(work, timeoutCh)
		if nil != err {
			log4go.ErrorLog("handler", "Batch|Wait|queue|FAIL|%v", err)
			if err == ERR_QUEUE_TIMEOUT && !timeoutClosed {
				//关闭这个channel
				close(timeoutCh)
				timeoutClosed = true
			}
		}
		wus = append(wus, wu)

	}
	//等待完成
	if nil != timeoutCh {
		for i := range wus {
			select {
			case <-timeoutCh:
				//超时了。。。。有多少返回多少
				for j := i; j < len(wus); j++ {
					//直接给结果
					wus[j].Cancel()
					if nil == wus[j].Err {
						wus[j].Err = ERR_QUEUE_CANCEL
					}
				}
				return wus, nil
			case <-wus[i].ch:
			}
		}
		return wus, nil
	} else {
		for i := range wus {
			<-wus[i].ch
		}
		return wus, nil
	}
}
