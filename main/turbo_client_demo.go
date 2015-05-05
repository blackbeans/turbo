package main

import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/packet"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func clientPacketDispatcher(rclient *client.RemotingClient, resp *packet.Packet) {
	rclient.Attach(resp.Opaque, resp.Data)
	// log.Printf("clientPacketDispatcher|%s\n", string(resp.Data))
}

func handshake(ga *client.GroupAuth, remoteClient *client.RemotingClient) (bool, error) {
	return true, nil
}

func main() {

	go func() {
		http.ListenAndServe(":13801", nil)

	}()

	// 重连管理器
	reconnManager := client.NewReconnectManager(false, -1, -1, handshake)

	clientManager := client.NewClientManager(reconnManager)

	rcc := turbo.NewRemotingConfig(
		"turbo-client:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	go func() {
		for {
			log.Println(rcc.FlowStat.Monitor())
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
			log.Printf("KiteClientManager|RECONNECT|%s|FAIL|%s\n", hostport, err)
			return nil, err
		}

		return conn, nil
	}("localhost:28888")

	remoteClient := client.NewRemotingClient(conn, clientPacketDispatcher, rcc)
	remoteClient.Start()

	auth := &client.GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, remoteClient)

	//echo command
	p := packet.NewPacket(1, []byte("echo"))

	//find a client
	tmp := clientManager.FindRemoteClients([]string{"a"}, func(groupid string, c *client.RemotingClient) bool {
		return false
	})

	ch := make(chan int, 1)
	for {
		ch <- 1
		go func() {

			//write command and wait for response
			_, err := tmp["a"][0].WriteAndGet(*p, 500*time.Millisecond)
			if nil != err {
				log.Printf("WAIT RESPONSE FAIL|%s\n", err)
			} else {
				// log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
			}
			<-ch
		}()
	}

	select {}

}
