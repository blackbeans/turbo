package server

import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"log"
	"net"
	"testing"
	"time"
)

var flow = turbo.NewRemotingFlow("turbo-server:localhost:28888")
var clientf = turbo.NewRemotingFlow("turbo-client:localhost:28888")

func clientPacketDispatcher(rclient *client.RemotingClient, resp *packet.Packet) {
	rclient.Attach(resp.Header.Opaque, resp.Data)
}

func packetDispatcher(rclient *client.RemotingClient, p *packet.Packet) {
	resp := packet.NewRespPacket(p.Header.Opaque, p.Header.CmdType, p.Data)
	//直接回写回去
	rclient.Write(*resp)
	flow.WriteFlow.Incr(1)
}

func handshake(ga *client.GroupAuth, remoteClient *client.RemotingClient) (bool, error) {
	return true, nil
}

func BenchmarkRemoteClient(t *testing.B) {

	rc := turbo.NewRemotingConfig(
		"turbo-server:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteServer := NewRemotionServer("localhost:28888", rc, packetDispatcher)
	remoteServer.ListenAndServer()

	conn, _ := dial("localhost:28888")

	// //重连管理器
	reconnManager := client.NewReconnectManager(false, -1, -1, handshake)

	clientManager := client.NewClientManager(reconnManager)

	rcc := turbo.NewRemotingConfig(
		"turbo-client:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteClient := client.NewRemotingClient(conn, func() codec.ICodec {
		return codec.LengthBasedCodec{
			MaxFrameLength: packet.MAX_PACKET_BYTES,
			SkipLength:     4}
	}, clientPacketDispatcher, rcc)
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

	t.SetParallelism(4)

	t.RunParallel(func(pb *testing.PB) {

		for pb.Next() {
			for i := 0; i < t.N; i++ {
				p := packet.NewPacket(1, []byte("echo"))
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

	})
}

//创建物理连接
func dial(hostport string) (*net.TCPConn, error) {
	//连接
	remoteAddr, err_r := net.ResolveTCPAddr("tcp4", hostport)
	if nil != err_r {
		log.Printf("KiteClientManager|RECONNECT|RESOLVE ADDR |FAIL|remote:%s\n", err_r)
		return nil, err_r
	}
	conn, err := net.DialTCP("tcp4", nil, remoteAddr)
	if nil != err {
		log.Printf("KiteClientManager|RECONNECT|%s|FAIL|%s\n", hostport, err)
		return nil, err
	}

	return conn, nil
}
