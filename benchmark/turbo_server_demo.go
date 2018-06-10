package main

import (
	"net/http"
	_ "net/http/pprof"
	"time"
	"turbo"
)

func handle(ctx *turbo.TContext) error{
	// log.Printf("packetDispatcher|WriteResponse|%s\n", string(p.Data))
	p := ctx.Message
	resp := turbo.NewRespPacket(p.Header.Opaque, p.Header.CmdType, p.Data)
	//直接回写回去
	ctx.Client.Write(*resp)
	return nil
}

func main() {

	go func() {
		http.ListenAndServe(":13800", nil)

	}()

	rc := turbo.NewTConfig(
		"turbo-server:localhost:28888",
		1000, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second, 160000)

	remoteServer := turbo.NewTServer("localhost:28888", rc, handle)
	remoteServer.ListenAndServer()
	select {}
}
