package turbo

import (
	"github.com/blackbeans/pool"
	"log"
	"net"
	"testing"
	"time"
)

var flow = NewRemotingFlow("turbo-server:localhost:28888", pool.New())
var clientf = NewRemotingFlow("turbo-client:localhost:28888", pool.New())

//开启server
func TestLineBaseServer(t *testing.T) {

	serConfig := NewTConfig(
		"turbo-server:localhost:28889",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second)

	server := NewTServerWithCodec("localhost:28889", serConfig, func() ICodec {
		return LengthBytesCodec{MaxFrameLength: MAX_PACKET_BYTES}
	}, func(ctx *TContext) error {
		p := ctx.Message
		resp := NewRespPacket(p.Header.Opaque, p.Header.CmdType, nil)
		resp.PayLoad = p.Data
		//直接回写回去
		ctx.Client.Write(*resp)
		flow.WriteFlow.Incr(1)
		return nil
	})
	server.ListenAndServer()

	conn, _ := dial("localhost:28889")

	// //重连管理器
	reconnManager := NewReconnectManager(false, -1, -1,
		func(ga *GroupAuth, remoteClient *TClient) (bool, error) {
			return true, nil
		})

	clientManager := NewClientManager(reconnManager)

	config := NewTConfig(
		"turbo-client:localhost:28889",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second)

	remoteClient := NewTClient(conn, func() ICodec {
		return LengthBytesCodec{MaxFrameLength: MAX_PACKET_BYTES}
	},
		func(ctx *TContext) error {
			ctx.Client.Attach(ctx.Message.Header.Opaque, ctx.Message.Data)
			return nil
		}, config)
	remoteClient.Start()

	auth := &GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, remoteClient)
	go func() {
		for {
			time.Sleep(1 * time.Second)
		}
	}()

	for i := 0; i < 10; i++ {
		p := NewPacket(1, nil)
		p.PayLoad = []byte("echo")
		p.Header.Opaque = 1
		tmp := clientManager.FindTClients([]string{"a"}, func(groupid string, c *TClient) bool {
			return false
		})
		resp, err := tmp["a"][0].WriteAndGet(*p, 500*time.Millisecond)
		clientf.WriteFlow.Incr(1)
		if nil != err {
			t.Fail()
			log.Printf("WAIT RESPONSE FAIL|%s\n", err)
		} else {
			log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
		}
	}

}

func BenchmarkRemoteClient(t *testing.B) {

	remoteServer := NewTServer("localhost:28888",
		NewTConfig(
			"turbo-server:localhost:28888",
			100, 16*1024,
			16*1024, 100, 100,
			10*time.Second),
		func(ctx *TContext) error {
			p := ctx.Message
			resp := NewRespPacket(p.Header.Opaque, p.Header.CmdType, nil)
			resp.PayLoad = p.Data
			//直接回写回去
			ctx.Client.Write(*resp)
			flow.WriteFlow.Incr(1)
			return nil
		})
	remoteServer.ListenAndServer()

	// //重连管理器
	reconnManager := NewReconnectManager(false, -1, -1,
		func(ga *GroupAuth, remoteClient *TClient) (bool, error) {
			return true, nil
		})

	clientManager := NewClientManager(reconnManager)

	conn, _ := dial("localhost:28888")
	remoteClient := NewTClient(conn,
		func() ICodec {
			return LengthBytesCodec{
				MaxFrameLength: MAX_PACKET_BYTES}
		},
		func(ctx *TContext) error {
			ctx.Client.Attach(ctx.Message.Header.Opaque, ctx.Message.Data)
			return nil
		}, NewTConfig(
			"turbo-server:localhost:28888",
			100, 16*1024,
			16*1024, 100, 100,
			10*time.Second))
	remoteClient.Start()

	auth := &GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, remoteClient)
	go func() {
		for {
			time.Sleep(1 * time.Second)
		}
	}()

	t.SetParallelism(8)

	t.RunParallel(func(pb *testing.PB) {

		for pb.Next() {
			for i := 0; i < t.N; i++ {
				p := NewPacket(1, nil)
				p.PayLoad = []byte("echo")
				tmp := clientManager.FindTClients([]string{"a"}, func(groupid string, c *TClient) bool {
					return false
				})

				_, err := tmp["a"][0].WriteAndGet(*p, 5*time.Second)
				clientf.WriteFlow.Incr(1)
				if nil != err {
					t.Fail()
					log.Printf("WAIT RESPONSE FAIL|%s\n", err)
				} else {
					//log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
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
