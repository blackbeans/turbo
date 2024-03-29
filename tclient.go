package turbo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

//网络层的client
type TClient struct {
	conn       *net.TCPConn
	localAddr  string
	remoteAddr string
	heartbeat  int64
	wchan      chan *Packet //response的channel
	s          *TSession
	dis        THandler //包处理函数
	codec      func() ICodec
	config     *TConfig
	authSecond int64 //授权成功时间
	ctx        context.Context
	closeFunc  context.CancelFunc
}

func NewTClient(parent context.Context,
	conn *net.TCPConn, codec func() ICodec, dis THandler,
	config *TConfig) *TClient {

	//初始化下cancel
	ctx, closeFunc := context.WithCancel(parent)
	//创建一个remotingcleint
	tclient := &TClient{
		heartbeat: 0,
		conn:      conn,
		dis:       dis,
		wchan:     make(chan *Packet, config.WriteChannelSize),
		config:    config,
		codec:     codec,
		ctx:       ctx,
		closeFunc: closeFunc}

	return tclient
}

func (self *TClient) RemoteAddr() string {
	return self.remoteAddr
}

func (self *TClient) LocalAddr() string {
	return self.localAddr
}

func (self *TClient) Idle() bool {
	return self.s.Idle()
}

//当接收到该链接的包
func (self *TClient) onMessage(msg Packet, err error) {

	//如果有错误，那么需要回给客户端错误包
	if nil != err {
		log.Errorf("TSession|onMessage|FAIL|%s|%v", self.remoteAddr, err)
		ctx := &TContext{
			Message: &msg,
			Client:  self,
			Err:     err,
		}
		err = self.dis(ctx)
		if nil != err {
			log.Errorf("TSession|onMessage|dis|FAIL|%s|%v", self.remoteAddr, err)
		}
	} else {
		p := &msg
		self.config.dispool.Queue(self.ctx, func(cctx context.Context) (interface{}, error) {
			//解析包
			message, err := self.codec().UnmarshalPayload(p)
			if nil != err {
				// 构造一个error的响应包
				log.Errorf("TSession|UnmarshalPayload|%s|FAIL|%v|bodyLen:%d",
					self.remoteAddr, err, msg.Header.BodyLen)
				ctx := &TContext{
					Message: p,
					Client:  self,
					Err:     err,
				}
				err = self.dis(ctx)
				return nil, nil
			}

			//强制设置payload
			p.PayLoad = message
			//创建上下文
			ctx := &TContext{
				Message: p,
				Client:  self,
			}
			//处理一下包
			err = self.dis(ctx)
			if nil != err {
				log.Errorf("TSession|onMessage|dis|FAIL|%s|%v", self.remoteAddr, err)
			}
			return nil, err
		})
	}
}

//启动当前的client
func (self *TClient) Start() {

	//启动session
	self.s = NewSession(self.conn, self.config, self.onMessage)
	//重新初始化
	laddr := self.conn.LocalAddr().(*net.TCPAddr)
	raddr := self.conn.RemoteAddr().(*net.TCPAddr)
	self.localAddr = fmt.Sprintf("%s:%d", laddr.IP, laddr.Port)
	self.remoteAddr = fmt.Sprintf("%s:%d", raddr.IP, raddr.Port)
	//启动读取
	self.s.Open()
	//启动异步写出
	self.asyncWrite()

	log.Infof("TClient|Start|SUCC|local:%s|remote:%s", self.LocalAddr(), self.RemoteAddr())
}

//同步发起ping的命令
func (self *TClient) Ping(heartbeat *Packet, timeout time.Duration) error {
	pong, err := self.WriteAndGet(*heartbeat, timeout)
	if nil != err {
		return err
	}
	version, ok := pong.(int64)
	if !ok {
		log.Warnf("TClient|Ping|Pong|ERROR TYPE |%s", pong)
		return ERR_PONG
	}
	self.updateHeartBeat(version)
	return nil
}

func (self *TClient) updateHeartBeat(version int64) {
	if version > self.heartbeat {
		self.heartbeat = version
	}
}

func (self *TClient) Pong(opaque uint32, version int64) {
	self.updateHeartBeat(version)
}

func (self *TClient) fillOpaque(p *Packet) uint32 {
	tid := p.Header.Opaque
	//只有在默认值没有赋值的时候才去赋值
	if tid <= 0 {
		id := self.config.RequestHolder.CurrentOpaque()
		p.Header.Opaque = id
		tid = id
	}

	return tid
}

//将结果attach到当前的等待回调chan
func (self *TClient) Attach(opaque uint32, obj interface{}) {
	defer func() {
		if err := recover(); nil != err {
			log.Errorf("TClient|Attach|FAIL|%s|%s", err, obj)
		}
	}()

	self.config.RequestHolder.Detach(opaque, obj)

}

//写数据并且得到相应
func (self *TClient) WriteAndGet(p Packet,
	timeout time.Duration) (interface{}, error) {

	pp := &p
	opaque := self.fillOpaque(pp)
	future := NewFuture(opaque, timeout, self.localAddr, self.ctx)
	tchan := self.config.RequestHolder.Attach(opaque, future)
	//写入完成之后的操作
	pp.OnComplete = func(err error) {
		if nil != err {
			log.Errorf("TClient|Write|OnComplete|ERROR|FAIL|%v|%s", err, string(pp.Data))
			future.Error(err)
			//生成一个错误的转发
			ctx := &TContext{
				Client:  self,
				Message: pp,
				Err:     err}
			self.dis(ctx)
		}
	}

	//写入队列
	select {
	case self.wchan <- pp:
	default:
		err := errors.New(fmt.Sprintf("WRITE CHANNLE [%s] FULL", self.remoteAddr))
		pp.OnComplete(err)
		future.Error(err)
	}
	resp, err := future.Get(tchan)
	return resp, err
}

//分组写入
func (self *TClient) GroupWriteAndGet(timeout time.Duration, packets ...Packet) ([]*Future, error) {

	futures := make([]*Future, 0, len(packets))
	for i := range packets {
		pp := &(packets[i])
		opaque := self.fillOpaque(pp)
		future := NewFuture(opaque, timeout, self.localAddr, self.ctx)

		self.config.RequestHolder.Attach(opaque, future)
		//写入完成之后的操作
		pp.OnComplete = func(err error) {
			if nil != err {
				log.Errorf("TClient|Write|OnComplete|ERROR|FAIL|%v|%s", err, string(pp.Data))
				future.Error(err)
				//生成一个错误的转发
				ctx := &TContext{
					Client:  self,
					Message: pp,
					Err:     err}
				self.dis(ctx)
			}
		}
		//写入队列
		select {
		case self.wchan <- pp:
		default:
			err := errors.New(fmt.Sprintf("WRITE CHANNLE [%s] FULL", self.remoteAddr))
			pp.OnComplete(err)
			future.Error(err)
		}
		futures = append(futures, future)
	}
	return futures, nil
}

//只是写出去
func (self *TClient) Write(p Packet) error {

	//写入完成之后的操作
	p.OnComplete = func(err error) {
		if nil != err {
			log.Errorf("TClient|Write|OnComplete|ERROR|FAIL|%v|%s", err, string(p.Data))
			//生成一个错误的转发
			ctx := &TContext{
				Client:  self,
				Message: &p,
				Err:     err}
			self.dis(ctx)
		}
	}

	//写入队列
	select {
	case self.wchan <- &p:
		return nil
	default:
		err := errors.New(fmt.Sprintf("WRITE CHANNLE [%s] FULL", self.remoteAddr))
		p.OnComplete(err)
		return err
	}
}

//写入响应
func (self *TClient) asyncWrite() {

	go func() {
		for !self.IsClosed() {

			tid, timeout := self.config.TW.AddTimer(1*time.Second, nil, nil)
			select {
			case p := <-self.wchan:
				//先读到数据，则取消定时
				self.config.TW.CancelTimer(tid)
				if nil != p {
					//这里坐下序列化，看下Body是否大于最大的包大小
					raw, err := self.codec().MarshalPayload(p)
					if nil != err {
						log.Errorf("TClient|asyncWrite|MarshalPayload|FAIL|%v|%+v",
							err, p.PayLoad)
						if nil != p.OnComplete {
							p.OnComplete(err)
						}
						continue
					} else if len(raw) > MAX_PACKET_BYTES {
						log.Errorf("TClient|asyncWrite|MarshalPayload|FAIL|MAX_PACKET_BYTES|%d|%d",
							len(raw), MAX_PACKET_BYTES)
						if nil != p.OnComplete {
							p.OnComplete(ERR_TOO_LARGE_PACKET)
						}
						continue
					} else {

						//设置数据
						p.Data = raw
						//其他的都OK
					}
					//批量写入
					err = self.s.Write(p)
					//链接是关闭的
					if nil != err {
						log.Errorf("TClient|asyncWrite|Write|FAIL|%v",
							err)
						self.s.Close()
						continue
					}
				}
			case <-timeout:
				//超时了
			}
		}
	}()
}

func (self *TClient) IsClosed() bool {
	return self.s.Closed()
}

func (self *TClient) Shutdown() {
	self.closeFunc()
	self.s.Close()
	log.Infof("TClient|Shutdown|%s...", self.RemoteAddr())
}
