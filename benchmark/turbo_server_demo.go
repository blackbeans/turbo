package main

import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/packet"
	"github.com/blackbeans/turbo/server"
	// "log"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func packetDispatcher(rclient *client.RemotingClient, p *packet.Packet) {
	// log.Printf("packetDispatcher|WriteResponse|%s\n", string(p.Data))
	resp := packet.NewRespPacket(p.Header.Opaque, p.Header.CmdType, p.Data)
	//直接回写回去
	rclient.Write(*resp)

}

func main() {

	go func() {
		http.ListenAndServe(":13800", nil)

	}()

	rc := turbo.NewRemotingConfig(
		"turbo-server:localhost:28888",
		1000, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second, 160000)

	remoteServer := server.NewRemotionServer("localhost:28888", rc, packetDispatcher)
	remoteServer.ListenAndServer()
	select {}
}
