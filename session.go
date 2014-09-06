package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/cuixin/goalg/queue"
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
	Sid            string       // 用户SessionId
	Uid            string       // 用户Uid
	RemoteAddr     string       // 远程连接地址
	ConnectTime    time.Time    // 连接时间
	LastPacketTime time.Time    // 最后一次发包时间，判定是否玩家已经超时离线
	PacketCount    int64        // 发送请求包的总数量
	writeLock      *sync.Mutex  // 下行数据的锁
	writeQueue     *queue.Queue `json:"-"` // 下行数据的队列
	Attachment     interface{}  `json:"-"` // 绑定的数据
	sync.Mutex
}

func (self *Session) PushQueue(v interface{}) {
	self.writeLock.Lock()
	self.writeQueue.Enqueue(v)
	self.writeLock.Unlock()
}

func (self *Session) RemoveQueue() []interface{} {
	var retQueue []interface{}
	self.writeLock.Lock()
	qLen := self.writeQueue.Len()
	if qLen > 0 {
		retQueue = make([]interface{}, 0, qLen)
		for {
			front := self.writeQueue.Dequeue()
			if front == nil {
				break
			}
			retQueue = append(retQueue, front)
		}
	}
	self.writeLock.Unlock()
	return retQueue
}

// 实现一个双向唯一Sid<->Uid
var this = &sessionManager{
	make(map[string]*Session, 16<<10), // 16384
	make(map[string]*Session, 16<<10),
	&sync.Mutex{},
}

type sessionManager struct {
	sidMaps map[string]*Session // SessionId ----> Session
	uidMaps map[string]*Session // Uid       ----> Session
	*sync.Mutex
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
	this.Lock()
	if oldSession, ok := this.sidMaps[sid]; ok {
		this.Unlock()
		return oldSession, false
	}
	if oldSession, ok := this.uidMaps[uid]; ok {
		this.Unlock()
		return oldSession, false
	}
	this.sidMaps[sid] = s
	this.uidMaps[uid] = s
	this.Unlock()
	return s, true
}

func GetAllSessionUids() []string {
	this.Lock()
	sLen := len(this.uidMaps)
	ret := make([]string, 0, sLen)
	for uid, _ := range this.uidMaps {
		ret = append(ret, uid)
	}
	this.Unlock()
	return ret
}

func ClearSession() {
	this.Lock()
	this.uidMaps = nil
	this.uidMaps = make(map[string]*Session, 16<<10)
	this.sidMaps = nil
	this.sidMaps = make(map[string]*Session, 16<<10)
	this.Unlock()
}

func GetSessionBySid(sid string) *Session {
	this.Lock()
	ret := this.sidMaps[sid]
	this.Unlock()
	return ret
}

func GetSessionByUid(uid string) *Session {
	this.Lock()
	ret := this.uidMaps[uid]
	this.Unlock()
	return ret
}

func GetSessionCount() int {
	this.Lock()
	ret := len(this.sidMaps)
	this.Unlock()
	return ret
}

func RemoveSession(s *Session) {
	this.Lock()
	delete(this.sidMaps, s.Sid)
	delete(this.uidMaps, s.Uid)
	this.Unlock()
}

func RemoveSessionBySid(sid string) *Session {
	this.Lock()
	s, ok := this.sidMaps[sid]
	if ok {
		delete(this.sidMaps, sid)
		delete(this.uidMaps, s.Uid)
	}
	this.Unlock()
	return s
}

func RemoveSessionByUid(uid string) *Session {
	this.Lock()
	s, ok := this.uidMaps[uid]
	if ok {
		delete(this.sidMaps, s.Sid)
		delete(this.uidMaps, uid)
	}
	this.Unlock()
	return s
}

// 缓存到本地文件
func DumpToFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	bytes, err := json.Marshal(this.sidMaps)
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
	this.sidMaps = nil
	this.uidMaps = nil
	this.sidMaps = make(map[string]*Session, 16<<10)
	this.uidMaps = make(map[string]*Session, 16<<10)

	json.Unmarshal(data, &this.sidMaps)
	l1 := len(this.sidMaps)
	for _, v := range this.sidMaps {
		v.writeLock = &sync.Mutex{}
		v.writeQueue = queue.New()
		this.uidMaps[v.Uid] = v
	}
	return l1, nil
}

var OnRecycled func(s *Session)

// 立即回收
func Recycle(timeout time.Duration) {
	this.Lock()
	now := time.Now()
	for _, v := range this.sidMaps {
		// expired
		if now.After(v.LastPacketTime.Add(timeout)) {
			// fmt.Println(v.Uid, "Expired")
			delete(this.sidMaps, v.Sid)
			delete(this.uidMaps, v.Uid)
			if OnRecycled != nil {
				OnRecycled(v)
			}
		}
	}
	this.Unlock()
}

// 启动回收机制
func StartRecycle(period time.Duration, timeout time.Duration) {
	go func() {
		c := time.Tick(period)
		for now := range c {
			this.Lock()
			for _, v := range this.sidMaps {
				// expired
				if now.After(v.LastPacketTime.Add(timeout)) {
					// fmt.Println(v.Uid, "Expired")
					delete(this.sidMaps, v.Sid)
					delete(this.uidMaps, v.Uid)
					if OnRecycled != nil {
						OnRecycled(v)
					}
				}
			}
			this.Unlock()
		}
	}()
}
