package turbo

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// FutureTask已经被取消执行了
var ErrFutureTaskCancelled = errors.New("future task cancelled")

// Deprecated: use ErrFutureTaskCancelled instead
var ERR_FUTURE_TASK_CANCELLED = ErrFutureTaskCancelled

//future task
// how to use:
//	ctx,cancel := context.WithTimeout(parentCtx,5 * time.Second)
//  task := NewFutureTask(ctx,func(ctx context.Context){.......})
//  go task.Run()
//  resp,err := task.Get()
type FutureTask struct {
	ctx        context.Context
	cancelFunc context.CancelFunc

	wg   *sync.WaitGroup
	once *sync.Once
	do   func(ctx context.Context) (interface{}, error)

	err    error
	result interface{}
}

func NewFutureTask(parentCtx context.Context, do func(ctx context.Context) (interface{}, error)) *FutureTask {
	ctx, cancelFunc := context.WithCancel(parentCtx)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &FutureTask{
		wg:         wg,
		once:       &sync.Once{},
		do:         do,
		ctx:        ctx,
		cancelFunc: cancelFunc,
	}
}

func (t *FutureTask) Run() {
	t.once.Do(func() {
		defer func() {
			if err := recover(); nil != err {
				t.err = fmt.Errorf("%v", err)
			}
			t.wg.Done()
		}()
		if nil != t.do {
			select {
			case <-t.ctx.Done():
				t.err = ErrFutureTaskCancelled
			default:
				t.result, t.err = t.do(t.ctx)
			}
		}
	})
}

//获取本次执行结果
func (t *FutureTask) Get() (interface{}, error) {
	t.wg.Wait()
	return t.result, t.err
}

//取消任务执行
//future直接结束了
func (t *FutureTask) Cancel() {
	//取消任务执行
	t.cancelFunc()
	t.Run()
}
