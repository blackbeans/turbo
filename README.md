##### turbo
turbo is a lightweight  remoting framework 
 
##### Install

go get github.com/blackbeans/turbo

##### quickstart

server：

```
import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/packet"
	"log"
)

func packetDispatcher(rclient *client.RemotingClient, p *packet.Packet) {
 //add your packet decode and encode codes , recommend using  pipeline or chain 
	resp := packet.NewRespPacket(p.Opaque, p.CmdType, p.Data)
	//write response
	rclient.Write(*resp)
	log.Printf("packetDispatcher|WriteResponse|%s\n", string(resp.Data))
}

func main() {
	rc := turbo.NewRemotingConfig(
		"turbo-server:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteServer = NewRemotionServer("localhost:28888", rc, packetDispatcher)
	remoteServer.ListenAndServer()
}
```

client:
```
import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/packet"
	"log"
)

func clientPacketDispatcher(rclient *client.RemotingClient, resp *packet.Packet) {
	rclient.Attach(resp.Opaque, resp.Data)
	log.Printf("clientPacketDispatcher|%s\n", string(resp.Data))
}

func handshake(ga *client.GroupAuth, remoteClient *client.RemotingClient) (bool, error) {
	return true, nil
}

func main() {
	//reconnector
	reconnManager := client.NewReconnectManager(false, -1, -1, handshake)

	clientManager = client.NewClientManager(reconnManager)

	rcc := turbo.NewRemotingConfig(
		"turbo-client:localhost:28888",
		1000, 16*1024,
		16*1024, 10000, 10000,
		10*time.Second, 160000)

	remoteClient := client.NewRemotingClient(conn, clientPacketDispatcher, rcc)
	remoteClient.Start()

	auth := &client.GroupAuth{}
	auth.GroupId = "a"
	auth.SecretKey = "123"
	clientManager.Auth(auth, remoteClient)

	//echo command
	p := packet.NewPacket(1, []byte("echo"))

	for i := 0; i < t.N; i++ {
		//find a client
		tmp := clientManager.FindRemoteClients([]string{"a"}, func(groupid string, c *client.RemotingClient) bool {
			return false
		})

		//write command and wait for response
		_, err := tmp["a"][0].WriteAndGet(*p, 500*time.Millisecond)
		if nil != err {
			log.Printf("WAIT RESPONSE FAIL|%s\n", err)
		} else {
			// log.Printf("WAIT RESPONSE SUCC|%s\n", string(resp.([]byte)))
		}
	}

}
```

