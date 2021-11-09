package turbo

import (
	"context"
	"sync"
)

//future task
type FutureTask struct {
	
	ctx        context.Context
	cancelFunc context.CancelFunc
	
	wg   *sync.WaitGroup
	once *sync.Once
	do   func(ctx context.Context) (interface{}, error)

	err    error
	result interface{}
}

func NewFutureTask(do func(ctx context.Context) (interface{}, error)) *FutureTask {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &FutureTask{
		wg:   wg,
		once: &sync.Once{},
		do:   do,
	}
}

func (self *FutureTask) Run(ctx context.Context) {
	self.once.Do(func() {
		self.ctx, self.cancelFunc = context.WithCancel(ctx)
		if nil != self.do {
			select {
			case <-ctx.Done():
			default:
				self.result, self.err = self.do(ctx)
			}

		}
		self.wg.Done()
	})
}

//获取本次执行结果
func (self *FutureTask) Get() (interface{}, error) {
	self.wg.Wait()
	return self.result, self.err
}


//取消任务执行
//future直接结束了
func (self *FutureTask) Cancel() {
	//取消任务执行
	self.cancelFunc()
}

