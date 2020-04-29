package main

import (
	"github.com/blackbeans/turbo"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"
)

func handle(ctx *turbo.TContext) error {
	// log.Printf("packetDispatcher|WriteResponse|%s\n", string(p.Data))
	p := ctx.Message
	resp := turbo.NewRespPacket(p.Header.Opaque, p.Header.CmdType, nil)
	resp.PayLoad = p.Data
	//直接回写回去
	ctx.Client.Write(*resp)
	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()*2 + 1)
	go func() {
		http.ListenAndServe(":13800", nil)

	}()

	rc := turbo.NewTConfig(
		"turbo-server:localhost:28888",
		20, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second,
		50*10000)

	go func() {
		for {
			log.Println(rc.FlowStat.Stat())
			time.Sleep(1 * time.Second)
		}
	}()

	remoteServer := turbo.NewTServer("localhost:28888", rc, handle)
	remoteServer.ListenAndServer()
	select {}
}
