package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"io"
	"os"
	"sync"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"
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
	Sid            string        // 用户SessionId
	Uid            string        // 用户Uid
	RemoteAddr     string        // 远程连接地址
	ConnectTime    time.Time     // 连接时间
	LastPacketTime time.Time     // 最后一次发包时间，判定是否玩家已经超时离线
	PacketCount    int64         // 发送请求包的总数量
	Attachment     interface{}   `msgpack:"-"` // 绑定的数据
	DownQueue      *SafeQueue    `msgpack:"-"` // 下行数据的队列
	MsgQueue       chan *Message `msgpack:"-"` // 消息
}

// 实现一个双向唯一Sid<->Uid
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sidMaps: make(map[string]*Session, 16<<10), // 16384
		uidMaps: make(map[string]*Session, 16<<10),
	}
}

// 消息
type Message struct {
	Data   interface{}   // 实际数据
	IsDown chan struct{} // 完成通知
}

type SessionManager struct {
	sync.Mutex
	sidMaps map[string]*Session // SessionId ----> Session
	uidMaps map[string]*Session // Uid       ----> Session
}

func (this *SessionManager) NewSession(uid, remoteAddr string) (*Session, bool) {
	sid := NewSessionId(32)
	nowTime := time.Now()
	s := &Session{
		Sid:            sid,
		Uid:            uid,
		RemoteAddr:     remoteAddr,
		ConnectTime:    nowTime,
		LastPacketTime: nowTime,
		PacketCount:    1,
		DownQueue:      NewSafeQueue(),
		MsgQueue:       make(chan *Message, 16),
	}
	this.Lock()
	if oldSession, ok := this.sidMaps[sid]; ok {
		sid = NewSessionId(32)
		if oldSession2, ok2 := this.sidMaps[sid]; ok2 {
			this.Unlock()
			return oldSession2, false
		} else {
			s.Sid = sid
			this.Unlock()
			return s, true
		}
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

func (this *SessionManager) GetAllSessionUids() []string {
	this.Lock()
	sLen := len(this.uidMaps)
	ret := make([]string, sLen)
	i := 0
	for uid, _ := range this.uidMaps {
		ret[i] = uid
		i++
	}
	this.Unlock()
	return ret
}

func (this *SessionManager) AllSessionDo(call func(*Session)) {
	for _, v := range this.sidMaps {
		this.Lock()
		call(v)
		this.Unlock()
	}
}

func (this *SessionManager) ClearSession() {
	this.Lock()
	this.uidMaps = nil
	this.uidMaps = make(map[string]*Session, 16<<10)
	this.sidMaps = nil
	this.sidMaps = make(map[string]*Session, 16<<10)
	this.Unlock()
}

func (this *SessionManager) GetSessionBySid(sid string) *Session {
	this.Lock()
	ret := this.sidMaps[sid]
	this.Unlock()
	return ret
}

func (this *SessionManager) GetSessionByUid(uid string) *Session {
	this.Lock()
	ret := this.uidMaps[uid]
	this.Unlock()
	return ret
}

func (this *SessionManager) GetSessionCount() int {
	this.Lock()
	ret := len(this.sidMaps)
	this.Unlock()
	return ret
}

func (this *SessionManager) RemoveSession(s *Session) {
	this.Lock()
	delete(this.sidMaps, s.Sid)
	delete(this.uidMaps, s.Uid)
	close(s.MsgQueue)
	this.Unlock()
}

func (this *SessionManager) RemoveSessionBySid(sid string) *Session {
	this.Lock()
	s, ok := this.sidMaps[sid]
	if ok {
		delete(this.sidMaps, sid)
		delete(this.uidMaps, s.Uid)
	}
	this.Unlock()
	return s
}

func (this *SessionManager) RemoveSessionByUid(uid string) *Session {
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
func (this *SessionManager) DumpToFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := msgpack.Marshal(this.sidMaps)
	if err != nil {
		return err
	}
	bytes_len := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes_len, uint32(len(bytes)))
	f.Write(bytes_len)
	f.Write(bytes)

	return nil
}

//  从本地文件读取Session
func (this *SessionManager) LoadFromFile(filePath string) (int, error) {
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

	msgpack.Unmarshal(data, &this.sidMaps)
	l1 := len(this.sidMaps)
	for _, v := range this.sidMaps {
		v.MsgQueue = make(chan *Message, 16)
		v.DownQueue = NewSafeQueue()
		this.uidMaps[v.Uid] = v
	}
	return l1, nil
}
