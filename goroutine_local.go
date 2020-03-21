package turbo
// 放弃了该方法及其的性能差放弃
//
import (
	"sync"
)


var goroutineLocal = &sync.Map{}

//获取当前goroutine attacted的数据
func GetCurrAttached() (interface{}, bool) {
	return nil,false
}

//切记切记。在使用完之后要做移除。否则会造成内存泄露
//调用 DetachCurrGo
func AttachToCurrGo(v interface{}) () {
	//goroutineLocal.Store(CurGoroutineID(), v)
}

//解除当前attach的数据防止泄露
func DetachFromCurrGo() {
	//goroutineLocal.Delete(CurGoroutineID())
}
