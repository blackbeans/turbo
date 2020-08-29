package turbo

import (
	log "github.com/blackbeans/log4go"
	"math/rand"
	"sort"
	"sync"
	"time"
)

//群组授权信息
type GroupAuth struct {
	GroupId, SecretKey string
	WarmingupSec       int //该分组的预热时间
}

func NewGroupAuth(groupId, secretKey string) *GroupAuth {
	return &GroupAuth{SecretKey: secretKey, GroupId: groupId}
}

//远程client管理器
type ClientManager struct {
	reconnectManager *ReconnectManager
	groupAuth        map[string] /*host:port*/ *GroupAuth
	groupClients     map[string] /*groupId*/ []*TClient
	allClients       map[string] /*host:port*/ *TClient
	lock             sync.RWMutex
}

func NewClientManager(reconnectManager *ReconnectManager) *ClientManager {

	cm := &ClientManager{
		groupAuth:        make(map[string]*GroupAuth, 10),
		groupClients:     make(map[string][]*TClient, 50),
		allClients:       make(map[string]*TClient, 100),
		reconnectManager: reconnectManager}
	go cm.evict()
	return cm
}

func (self *ClientManager) evict() {
	log.Info("ClientManager|evict...")
	tick := time.NewTicker(1 * time.Minute)
	for {
		clients := self.ClientsClone()
		for _, c := range clients {
			if c.IsClosed() {
				//可能会删除连接，如果不开启重连策略的话
				self.SubmitReconnect(c)
			}
		}
		<-tick.C
	}
}

//connection numbers
func (self *ClientManager) ConnNum() int32 {
	clients := self.ClientsClone()
	return int32(len(clients))
}

//验证是否授权
func (self *ClientManager) Validate(client *TClient) bool {
	self.lock.RLock()
	defer self.lock.RUnlock()
	_, auth := self.groupAuth[client.RemoteAddr()]
	return auth
}

func (self *ClientManager) Auth(auth *GroupAuth, client *TClient) bool {
	self.lock.Lock()
	defer self.lock.Unlock()

	cs, ok := self.groupClients[auth.GroupId]
	if !ok {
		cs = make([]*TClient, 0, 50)
	}
	//创建remotingClient
	//增加授权时间的秒数
	client.authSecond = time.Now().Unix()
	self.groupAuth[client.RemoteAddr()] = auth
	self.groupClients[auth.GroupId] = append(cs, client)
	self.allClients[client.RemoteAddr()] = client
	return true
}

func (self *ClientManager) ClientsClone() map[string]*TClient {
	self.lock.RLock()
	defer self.lock.RUnlock()
	clone := make(map[string]*TClient, len(self.allClients))
	for k, v := range self.allClients {
		clone[k] = v
	}
	return clone
}

//返回group到IP的对应关系
func (self *ClientManager) CloneGroups() map[string][]string {
	self.lock.RLock()
	defer self.lock.RUnlock()
	clone := make(map[string][]string, len(self.groupClients))
	for k, v := range self.groupClients {
		clients := make([]string, 0, len(v))
		for _, c := range v {
			clients = append(clients, c.RemoteAddr())
		}
		sort.Strings(clients)
		clone[k] = clients
	}
	return clone
}

func (self *ClientManager) DeleteClients(hostports ...string) {
	self.lock.Lock()
	defer self.lock.Unlock()
	for _, hostport := range hostports {
		//取消重连
		self.removeClient(hostport)
		//取消重连任务
		self.reconnectManager.cancel(hostport)

	}
}

func (self *ClientManager) removeClient(hostport string) {

	ga, ok := self.groupAuth[hostport]
	if ok {
		//删除分组
		delete(self.groupAuth, hostport)
		//删除group中的client
		gc, ok := self.groupClients[ga.GroupId]
		if ok {
			for i, cli := range gc {
				if cli.RemoteAddr() == hostport {
					self.groupClients[ga.GroupId] = append(gc[0:i], gc[i+1:]...)
					break
				}
			}
		}

		//判断groupClient中是否还存在连接，不存在则直接移除该分组
		gc, ok = self.groupClients[ga.GroupId]
		if ok && len(gc) <= 0 {
			//移除group2Clients
			delete(self.groupClients, ga.GroupId)
			log.InfoLog("stdout", "ClientManager|removeClient|EmptyGroup|%s...", ga.GroupId)
		}
	}

	//删除hostport->client的对应关系
	c, ok := self.allClients[hostport]
	if ok && nil != c {
		c.Shutdown()
		delete(self.allClients, hostport)
	}
	log.InfoLog("stdout", "ClientManager|removeClient|%s...%d", hostport, len(self.allClients))
}

func (self *ClientManager) SubmitReconnect(c *TClient) {

	self.lock.Lock()
	defer self.lock.Unlock()
	ga, ok := self.groupAuth[c.RemoteAddr()]
	//如果重连则提交重连任务,并且该分组存在该机器则重连
	if ok && self.reconnectManager.allowReconnect {

		self.reconnectManager.submit(c, ga, func(addr string) {
			//重连任务失败完成后的hook,直接移除该机器
			self.DeleteClients(addr)
		})
		return

	} else if ok {
		//不需要重连的直接删除掉连接,或者分组不存在则直接删除
		self.removeClient(c.RemoteAddr())
	}

}

//查找remotingclient
//可能是已经关闭的状态
func (self *ClientManager) FindTClient(hostport string) *TClient {
	self.lock.RLock()
	defer self.lock.RUnlock()
	// log.Printf("ClientManager|FindTClient|%s|%s\n", hostport, self.allClients)
	rclient, ok := self.allClients[hostport]
	if !ok {
		return nil
	}

	return rclient
}

//查找匹配的groupids
func (self *ClientManager) FindTClients(groupIds []string, filter func(groupId string, rc *TClient) bool) map[string][]*TClient {
	clients := make(map[string][]*TClient, 10)
	closedClients := make(map[string]*TClient, 2)
	self.lock.RLock()
	for _, gid := range groupIds {
		if len(self.groupClients[gid]) <= 0 {
			continue
		}
		//按groupId来获取remoteclient
		gclient, ok := clients[gid]
		if !ok {
			gclient = make([]*TClient, 0, 10)
		}

		for _, c := range self.groupClients[gid] {

			if c.IsClosed() {
				closedClients[c.remoteAddr] = c
				continue
			}
			//如果当前client处于非关闭状态并且没有过滤则入选
			if !filter(gid, c) {

				//判断是否在预热周期内，预热周期内需要逐步放量
				auth, ok := self.groupAuth[c.remoteAddr]
				if ok && auth.WarmingupSec > 0 {

					//如果当前时间和授权时间差与warmingup需要的时间几率按照100%计算的比例
					//小于等于随机100出来的数据那么久可以选取
					rate := int((time.Now().Unix() - c.authSecond) * 100 / int64(auth.WarmingupSec))
					if rate < 100 && rand.Intn(100) > rate {
						continue
					}
				}

				gclient = append(gclient, c)
			}
		}
		clients[gid] = gclient
	}
	self.lock.RUnlock()

	//删除掉关掉的clients
	if len(closedClients) > 0 {
		for _, c := range closedClients {
			self.SubmitReconnect(c)
		}
	}

	// log.Printf("Find clients result |%s|%s\n", clients, self.groupClients)
	return clients
}

func (self *ClientManager) Shutdown() {
	self.reconnectManager.stop()
	for _, c := range self.allClients {
		c.Shutdown()
	}
	log.Info("ClientManager|Shutdown....")
}
