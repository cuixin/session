package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/cuixin/goalg/queue"
	"github.com/vmihailenco/msgpack"
	"io"
	"os"
	"sync"
	"time"
)

func NewSessionId(size int) string {
	k := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return ""
	} else {
		return base64.StdEncoding.EncodeToString(k)
	}
}

// 用户会话
type Session struct {
	Sid            string    // 用户SessionId
	Uid            string    // 用户Uid
	RemoteAddr     string    // 远程连接地址
	ConnectTime    time.Time // 连接时间
	LastPacketTime time.Time // 最后一次发包时间，判定是否玩家已经超时离线
	PacketCount    int64     // 发送请求包的总数量
	sync.Mutex
	writeLock  *sync.Mutex  // 下行数据的锁
	writeQueue *queue.Queue // 下行数据的队列
}

func (self *Session) PushQueue(v interface{}) {
	self.writeLock.Lock()
	self.writeQueue.Enqueue(v)
	self.writeLock.Unlock()
}

func (self *Session) RemoveQueue() []interface{} {
	self.writeLock.Lock()
	retQueue := make([]interface{}, 0)
	for {
		front := self.writeQueue.Dequeue()
		if front == nil { // empty
			break
		}
		retQueue = append(retQueue, front)
	}
	self.writeLock.Unlock()
	return retQueue
}

// 实现一个双向唯一Sid<->Uid
var this = &sessionManager{
	make(map[string]*Session, 16<<10), // 16384
	make(map[string]*Session, 16<<10),
	sync.RWMutex{},
}

type sessionManager struct {
	sidMaps map[string]*Session // SessionId ----> Session
	uidMaps map[string]*Session // Uid       ----> Session
	mu      sync.RWMutex
}

func NewSession(sid, uid, remoteAddr string) (*Session, bool) {
	nowTime := time.Now()
	s := &Session{
		Sid:            sid,
		Uid:            uid,
		RemoteAddr:     remoteAddr,
		ConnectTime:    nowTime,
		LastPacketTime: nowTime,
		PacketCount:    1,
		writeLock:      &sync.Mutex{},
		writeQueue:     queue.New(),
	}
	this.mu.Lock()
	if oldSession, ok := this.sidMaps[sid]; ok {
		this.mu.Unlock()
		return oldSession, false
	}
	if oldSession, ok := this.uidMaps[uid]; ok {
		this.mu.Unlock()
		return oldSession, false
	}
	this.sidMaps[sid] = s
	this.uidMaps[uid] = s
	this.mu.Unlock()
	return s, true
}

func GetSessionBySid(sid string) *Session {
	this.mu.RLock()
	ret := this.sidMaps[sid]
	this.mu.RUnlock()
	return ret
}

func GetSessionByUid(uid string) *Session {
	this.mu.RLock()
	ret := this.uidMaps[uid]
	this.mu.RUnlock()
	return ret
}

func GetSessionCount() int {
	this.mu.RLock()
	ret := len(this.sidMaps)
	this.mu.RUnlock()
	return ret
}

func RemoveSession(s *Session) {
	this.mu.Lock()
	delete(this.sidMaps, s.Sid)
	delete(this.uidMaps, s.Uid)
	this.mu.Unlock()
}

func RemoveSessionBySid(sid string) *Session {
	this.mu.Lock()
	s, ok := this.sidMaps[sid]
	if ok {
		delete(this.sidMaps, sid)
		delete(this.uidMaps, s.Uid)
	}
	this.mu.Unlock()
	return s
}

func RemoveSessionByUid(uid string) *Session {
	this.mu.Lock()
	s, ok := this.uidMaps[uid]
	if ok {
		delete(this.sidMaps, s.Sid)
		delete(this.uidMaps, uid)
	}
	this.mu.Unlock()
	return s
}

// 缓存到本地文件
func DumpToFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	bytes, err := msgpack.Marshal(this.sidMaps, this.uidMaps)
	bytes_len := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes_len, uint32(len(bytes)))
	f.Write(bytes_len)
	f.Write(bytes)
	return nil
}

// // 从本地文件读取Session
func LoadFromFile(filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return -1, err
	}
	defer f.Close()
	len_bytes := make([]byte, 4)
	f.Read(len_bytes)
	data := make([]byte, binary.BigEndian.Uint32(len_bytes))
	f.Read(data)
	msgpack.Unmarshal(data, &this.sidMaps, &this.uidMaps)
	l1 := len(this.sidMaps)
	l2 := len(this.uidMaps)
	for _, v := range this.sidMaps {
		v.writeLock = &sync.Mutex{}
		v.writeQueue = queue.New()
	}

	for _, v := range this.uidMaps {
		v.writeLock = &sync.Mutex{}
		v.writeQueue = queue.New()
	}
	if l1 != l2 {
		return -1, fmt.Errorf("Cannot equal maps", l1, l2)
	}
	return l1, nil
}

var OnRecycled func(s *Session)

// 立即回收
func Recycle(timeout time.Duration) {
	this.mu.Lock()
	now := time.Now()
	for _, v := range this.sidMaps {
		// expired
		if now.After(v.LastPacketTime.Add(timeout)) {
			// fmt.Println(v.Uid, "Expired")
			delete(this.sidMaps, v.Sid)
			delete(this.uidMaps, v.Uid)
			if OnRecycled != nil {
				OnRecycled(s)
			}
		}
	}
	this.mu.Unlock()
}

// 启动回收机制
func StartRecycle(period time.Duration, timeout time.Duration) {
	go func() {
		c := time.Tick(period)
		for now := range c {
			this.mu.Lock()
			for _, v := range this.sidMaps {
				// expired
				if now.After(v.LastPacketTime.Add(timeout)) {
					// fmt.Println(v.Uid, "Expired")
					delete(this.sidMaps, v.Sid)
					delete(this.uidMaps, v.Uid)
					if OnRecycled != nil {
						OnRecycled(s)
					}
				}
			}
			this.mu.Unlock()
		}
	}()
}
