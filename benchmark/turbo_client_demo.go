package main

import (
	"context"
	"github.com/blackbeans/turbo"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func onMessage(ctx *turbo.TContext) error {
	resp := ctx.Message
	ctx.Client.Attach(resp.Header.Opaque, resp.Data)
	// log.Printf("onMessage|%s\n", string(resp.Data))
	return nil
}

func main() {

	go func() {
		http.ListenAndServe(":13802", nil)

	}()

	// 重连管理器
	reconnManager := turbo.NewReconnectManager(false, -1, -1,
		func(ga *turbo.GroupAuth, remoteClient *turbo.TClient) (bool, error) {
			return true, nil
		})

	clientManager := turbo.NewClientManager(reconnManager)

	rcc := turbo.NewTConfig(
		"turbo-client:localhost:28888",
		10, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second,
		50*10000)

	go func() {
		for {
			log.Println(rcc.FlowStat.Stat())
			time.Sleep(1 * time.Second)
		}
	}()

	//创建物理连接
	conn, _ := func(hostport string) (*net.TCPConn, error) {
		//连接
		remoteAddr, err_r := net.ResolveTCPAddr("tcp4", hostport)
		if nil != err_r {
			log.Printf("KiteClientManager|RECONNECT|RESOLVE ADDR |FAIL|remote:%s\n", err_r)
			return nil, err_r
		}
		conn, err := net.DialTCP("tcp4", nil, remoteAddr)
		if nil != err {
			log.Printf("KiteClientManager|RECONNECT|%s|FAIL|%s", hostport, err)
			return nil, err
		}

		return conn, nil
	}("localhost:28888")

	client := turbo.NewTClient(context.Background(), conn,
		func() turbo.ICodec {
			return turbo.LengthBytesCodec{
				MaxFrameLength: turbo.MAX_PACKET_BYTES}
		}, onMessage, rcc)
	client.Start()

	auth := &turbo.GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, client)

	//echo command
	p := turbo.NewPacket(1, nil)
	p.PayLoad = []byte("echo")

	//find a client
	tmp := clientManager.FindTClients([]string{"a"}, func(groupid string, c *turbo.TClient) bool {
		return false
	})

	for i := 0; i < 100; i++ {
		go func() {
			for {
				//write command and wait for response
				_, err := tmp["a"][0].WriteAndGet(*p, 5*time.Second)
				if nil != err {
					log.Printf("WAIT RESPONSE FAIL|%s\n", err)
					break
				} else {
					// log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
				}

			}
		}()
	}

	select {}

}
