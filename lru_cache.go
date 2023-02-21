package turbo

import (
	"context"
	"sync/atomic"
	"time"
)

type Entry struct {
	Timerid uint32
	Value   interface{}
}

//lru的cache
type LRUCache struct {
	ctx   context.Context
	hit   uint64
	total uint64
	cache *cache
	tw    *TimerWheel
}

//LRU的cache
func NewLRUCache(
	ctx context.Context,
	maxcapacity int,
	expiredTw *TimerWheel,
	OnEvicted func(k, v interface{})) *LRUCache {

	cache := New(maxcapacity)
	cache.OnEvicted = func(key Key, value interface{}) {
		vv := value.(Entry)
		if nil != OnEvicted {
			OnEvicted(key.(interface{}), vv.Value)
		}
		//因为已经淘汰，所以清理掉定时过期的任务
		expiredTw.CancelTimer(vv.Timerid)

	}

	lru := &LRUCache{
		ctx:   ctx,
		cache: cache,
		tw:    expiredTw}
	return lru
}

//本时间段的命中率
func (self *LRUCache) HitRate() (int, int) {
	currHit := self.hit
	currTotal := self.total

	if currTotal <= 0 {
		return 0, self.Length()
	}
	return int(currHit * 100 / currTotal), self.Length()
}

//获取
func (self *LRUCache) Get(key interface{}) (interface{}, bool) {
	atomic.AddUint64(&self.total, 1)
	if v, ok := self.cache.Get(key); ok {
		atomic.AddUint64(&self.hit, 1)
		return v.(Entry).Value, true
	}
	return nil, false
}

//增加元素
func (self *LRUCache) Put(key, v interface{}, ttl time.Duration) chan time.Time {
	entry := Entry{Value: v}
	var ttlChan chan time.Time
	if ttl > 0 {
		//超时过期或者取消的时候也删除
		if nil != self.tw {
			//如果数据存在，则更新过期时间
			if v, ok := self.cache.Get(key); ok {
				if exist, ok := v.(Entry); ok {
					//先取消掉过期淘汰定时,然后再重新设置定时器
					self.tw.CancelTimer(exist.Timerid)
				}
			}
			timerid, ch := self.tw.AddTimer(ttl, func(tid uint32, t time.Time) {
				//直接移除就可以，cache淘汰会调用OnEvicted
				self.cache.Remove(key)
			}, func(tid uint32, t time.Time) {

			})
			entry.Timerid = timerid
			ttlChan = ch
		}
	}
	self.cache.Add(key, entry)
	return ttlChan
}

//移除元素
func (self *LRUCache) Remove(key interface{}) interface{} {
	entry := self.cache.Remove(key)
	if nil != entry {
		e := entry.(Entry)
		if nil != self.tw {
			self.tw.CancelTimer(e.Timerid)
		}
		return e.Value
	}

	return nil
}

//是否包含当前的KEY
func (self *LRUCache) Contains(key interface{}) bool {
	if _, ok := self.cache.Get(key); ok {
		return ok
	}
	return false
}

//c length
func (self *LRUCache) Length() int {
	return self.cache.Len()
}

func (self *LRUCache) Iterator(do func(k, v interface{}) error) {
	self.cache.Iterator(func(k, v interface{}) error {
		entry := v.(Entry)
		return do(k, entry.Value)
	})
}
