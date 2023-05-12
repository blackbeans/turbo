package turbo

import (
	"context"
	"github.com/blackbeans/logx"
	"net"
	"runtime"
	"time"
)

//turbo日志
var log = logx.GetLogger("turbo")

type TServer struct {
	ctx        context.Context
	cancel     context.CancelFunc
	hostport   string
	keepalive  time.Duration
	stopChan   chan bool
	isShutdown bool
	onMessage  THandler
	config     *TConfig
	codec      func() ICodec
}

func NewTServer(hostport string, config *TConfig,
	onMessage THandler) *TServer {

	runtime.GOMAXPROCS(runtime.NumCPU() + 1)
	server := &TServer{
		hostport:   hostport,
		stopChan:   make(chan bool, 1),
		onMessage:  onMessage,
		isShutdown: false,
		config:     config,
		keepalive:  5 * time.Minute,
		codec: func() ICodec {
			return LengthBytesCodec{
				MaxFrameLength: MAX_PACKET_BYTES}

		}}

	server.ctx, server.cancel = context.WithCancel(context.Background())
	return server
}

func NewTServerWithCodec(hostport string, config *TConfig, codec func() ICodec,
	onMessage THandler) *TServer {

	//设置为8个并发
	runtime.GOMAXPROCS(runtime.NumCPU() + 1)

	server := &TServer{
		hostport:   hostport,
		stopChan:   make(chan bool, 1),
		onMessage:  onMessage,
		isShutdown: false,
		config:     config,
		keepalive:  5 * time.Minute,
		codec:      codec}
	server.ctx, server.cancel = context.WithCancel(context.Background())
	return server
}

func (self *TServer) ListenAndServer() error {

	addr, err := net.ResolveTCPAddr("tcp4", self.hostport)
	if nil != err {
		log.Errorf("TServer|ADDR|FAIL|%s", self.hostport)
		return err
	}

	listener, err := net.ListenTCP("tcp4", addr)
	if nil != err {
		log.Errorf("TServer|ListenTCP|FAIL|%v|%s", err, addr)
		return err
	}

	stopListener := &StoppedListener{listener, self.stopChan, make(chan *net.TCPConn), self.keepalive}

	//开始服务获取连接
	go self.serve(stopListener)
	return nil

}

//networkstat
func (self *TServer) NetworkStat() NetworkStat {
	return self.config.FlowStat.Stat()
}

//列出来客户端
func (self *TServer) ListClients() []string {
	clients := make([]string, 0, 10)
	self.config.FlowStat.Clients.Range(func(key, value interface{}) bool {
		clients = append(clients, key.(string))
		return true
	})

	return clients
}

func (self *TServer) serve(l *StoppedListener) error {
	for !self.isShutdown {
		conn, err := l.Accept()
		if nil != err {
			log.Errorf("TServer|serve|AcceptTCP|FAIL|%s", err)
			continue
		} else {
			//创建remotingClient对象
			tclient := NewTClient(self.ctx, conn, self.codec, self.onMessage, self.config)
			tclient.Start()
		}
	}
	return nil
}

func (self *TServer) Shutdown() {
	self.isShutdown = true
	close(self.stopChan)
	self.cancel()
	log.Infof("TServer|Shutdown...")
}
