package turbo

import (
	"testing"
	"time"
	"fmt"
	"math/rand"
)

//var tw = NewTimerWheel(100* time.Millisecond,10)
var c = NewLRUCache(10000,tw,
	func(k, v interface{}) {
		//fmt.Printf("OnEvnict %v %v\n",k,v)
	})

type Demo struct{
	Uid string
}

func TestLRUCache_Put(t *testing.T) {
	c.Put("100777", Demo{Uid:"100777"},10 * time.Second)
	time.Sleep(5 * time.Second)
	v,exist := c.Get("100777")
	if !exist{
		t.FailNow()
	}
	t.Log(v)
	time.Sleep(11 * time.Second)
	v,exist = c.Get("100777")
	if exist{
		t.FailNow()
	}

}

func TestLRUCache_Remove(t *testing.T) {
	c.Put("100777", Demo{Uid:"100777"},10 * time.Second)
	time.Sleep(5 * time.Second)
	v,exist := c.Get("100777")
	if !exist{
		t.FailNow()
	}
	t.Log(v)
	v  = c.Remove("100777")
	t.Log(v)
	v,exist = c.Get("100777")
	if exist{
		t.FailNow()
	}
	t.Log(v)

}

func TestLRUCache_Contains(t *testing.T) {
	c.Put("100777", Demo{Uid:"100777"},10 * time.Second)
	time.Sleep(5 * time.Second)
	exist := c.Contains("100777")
	if !exist{
		t.FailNow()
	}

	c.Remove("100777")

	exist = c.Contains("100777")
	if exist{
		t.FailNow()
	}
}


func BenchmarkLRUCache_Put(pb *testing.B) {
	pb.StopTimer()
	for i:= 0;i<1000;i++ {
		uid := fmt.Sprintf("10077%d",i)
		c.Put(uid, Demo{Uid: uid}, 10*time.Second)
	}

	go func(){
		for{
			time.Sleep(1 * time.Second)
			fmt.Printf("hit:%d\n", c.HitRate())
		}
	}()
	pb.StartTimer()
	pb.RunParallel(func(pb *testing.PB) {
		rand.Seed(time.Now().UnixNano())
		for pb.Next(){
			id := rand.Intn(10000)
			c.Get(fmt.Sprintf("10077%d",id))
		}
	})
}
