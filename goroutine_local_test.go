package turbo

import "testing"

func TestCurGoroutineID(t *testing.T) {
	goid := CurGoroutineID()
	t.Log(goid)
}

//
func TestGoroutineLocal(t *testing.T) {

	_,ok:= GetCurrAttached()
	if ok{
		t.FailNow()
	}

	AttachToCurrGo("a")

	v,ok := GetCurrAttached()
	if  !ok {
		t.FailNow()
	}

	val ,ok:= v.(string)
	if !ok || val != "a"{
		t.FailNow()
	}

	DetachFromCurrGo()

	v,ok = GetCurrAttached()
	if  ok {
		t.FailNow()
	}
}
