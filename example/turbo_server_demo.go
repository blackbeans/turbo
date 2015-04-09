package example

import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/packet"
	"github.com/blackbeans/turbo/server"
	"log"
	"time"
)

func packetDispatcher(rclient *client.RemotingClient, p *packet.Packet) {

	resp := packet.NewRespPacket(p.Opaque, p.CmdType, p.Data)
	//直接回写回去
	rclient.Write(*resp)
	log.Printf("packetDispatcher|WriteResponse|%s\n", string(resp.Data))
}

func main() {
	rc := turbo.NewRemotingConfig(
		"turbo-server:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteServer := server.NewRemotionServer("localhost:28888", rc, packetDispatcher)
	remoteServer.ListenAndServer()
}
