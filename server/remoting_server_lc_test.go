package server

import (
	// "fmt"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"log"
	"testing"
	"time"
)

func TestLineBaseServer(t *testing.T) {

	rc := turbo.NewRemotingConfig(
		"turbo-server:localhost:28889",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteServer := NewRemotionServerWithCodec("localhost:28889", rc, func() codec.ICodec {
		return codec.LineBasedCodec{MaxFrameLength: packet.MAX_PACKET_BYTES}
	}, func(rclient *client.RemotingClient, p *packet.Packet) {
		resp := packet.NewRespPacket(p.Header.Opaque, p.Header.CmdType, append(p.Data, []byte{'\r', '\n'}...))
		//直接回写回去
		rclient.Write(*resp)
		// fmt.Printf("----------server:%s\t%s\t%s\n", resp, rclient.RemoteAddr(), err)
		flow.WriteFlow.Incr(1)
	})
	remoteServer.ListenAndServer()

	conn, _ := dial("localhost:28889")

	// //重连管理器
	reconnManager := client.NewReconnectManager(false, -1, -1, handshake)

	clientManager := client.NewClientManager(reconnManager)

	rcc := turbo.NewRemotingConfig(
		"turbo-client:localhost:28889",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteClient := client.NewRemotingClient(conn, func() codec.ICodec {
		return codec.LineBasedCodec{MaxFrameLength: packet.MAX_PACKET_BYTES}
	}, func(rclient *client.RemotingClient, resp *packet.Packet) {
		// fmt.Printf("----------client:%s\n", resp)
		rclient.Attach(1, resp.Data)

	}, rcc)
	remoteClient.Start()

	auth := &client.GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, remoteClient)
	go func() {
		for {
			time.Sleep(1 * time.Second)
		}
	}()

	for i := 0; i < 1000; i++ {
		p := packet.NewPacket(1, []byte("echo\n"))
		p.Header.Opaque = 1
		tmp := clientManager.FindRemoteClients([]string{"a"}, func(groupid string, c *client.RemotingClient) bool {
			return false
		})
		_, err := tmp["a"][0].WriteAndGet(*p, 500*time.Millisecond)
		clientf.WriteFlow.Incr(1)
		if nil != err {
			t.Fail()
			log.Printf("WAIT RESPONSE FAIL|%s\n", err)
		} else {
			// log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
		}
	}

}
