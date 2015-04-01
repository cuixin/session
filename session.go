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
    mu sync.Mutex `msgpack:"-"`
    Sid            string        // 用户SessionId
	Uid            string        // 用户Uid
	RemoteAddr     string        // 远程连接地址
	ConnectTime    time.Time     // 连接时间
	LastPacketTime time.Time     // 最后一次发包时间，判定是否玩家已经超时离线
	PacketCount    int64         // 发送请求包的总数量
	Attachment     interface{}   `msgpack:"-"` // 绑定的数据
	DownQueue      *SafeQueue    `msgpack:"-"` // 下行数据的队列
	msgQ           chan *Message `msgpack:"-"` // 消息队列
    isClosed       bool
}

func (this *Session) closeMsgQ() {
    this.mu.Lock()
    if !this.isClosed {
        this.isClosed = true
    }
    close(this.msgQ)
    this.mu.Unlock()
}

// 发送消息
func (this *Session) SendMessage(msg *Message) bool {
    isSend := false
    this.mu.Lock()
    if !this.isClosed {
        isSend = true
        this.msgQ <- msg
        if msg.IsDown != nil {
            msg.IsDown <- struct{}{}
        }
    }
    this.mu.Unlock()
    return isSend
}

func (this *Session) GetMsgQ() <-chan *Message {
    return this.msgQ
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
	IsDown chan struct{} `msgpack:"-"` // 完成通知
}

type SessionManager struct {
	mu sync.Mutex
	sidMaps map[string]*Session // SessionId ----> Session
	uidMaps map[string]*Session // Uid       ----> Session
}

func (this *SessionManager) NewSession(uid, remoteAddr string) (*Session, bool) {
	sid := NewSessionId(32)
	nowTime := time.Now()
	s := &Session{
        mu : sync.Mutex{},
		Sid:            sid,
		Uid:            uid,
		RemoteAddr:     remoteAddr,
		ConnectTime:    nowTime,
		LastPacketTime: nowTime,
		PacketCount:    1,
		DownQueue:      NewSafeQueue(),
		msgQ:           make(chan *Message, 16),
        isClosed:       false,
	}
	this.mu.Lock()
	if oldSession, ok := this.sidMaps[sid]; ok {
		sid = NewSessionId(32)
		if oldSession2, ok2 := this.sidMaps[sid]; ok2 {
			this.mu.Unlock()
			return oldSession2, false
		} else {
			s.Sid = sid
			this.sidMaps[sid] = s
			this.uidMaps[uid] = s
			this.mu.Unlock()
			return s, true
		}
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

func (this *SessionManager) GetAllSessionUids() []string {
    this.mu.Lock()
    sLen := len(this.uidMaps)
	ret := make([]string, sLen)
	i := 0
	for uid, _ := range this.uidMaps {
		ret[i] = uid
		i++
	}
	this.mu.Unlock()
	return ret
}

func (this *SessionManager) GetAllSessionSids() []string {
	this.mu.Lock()
	sLen := len(this.sidMaps)
	ret := make([]string, sLen)
	i := 0
	for sid, _ := range this.sidMaps {
		ret[i] = sid
		i++
	}
	this.mu.Unlock()
	return ret
}

func (this *SessionManager) ClearSession() {
	this.mu.Lock()
	this.uidMaps = nil
	this.uidMaps = make(map[string]*Session, 16<<10)
	this.sidMaps = nil
	this.sidMaps = make(map[string]*Session, 16<<10)
	this.mu.Unlock()
}

func (this *SessionManager) GetSessionBySid(sid string) *Session {
	this.mu.Lock()
	ret := this.sidMaps[sid]
	this.mu.Unlock()
	return ret
}

func (this *SessionManager) GetSessionByUid(uid string) *Session {
	this.mu.Lock()
	ret := this.uidMaps[uid]
	this.mu.Unlock()
	return ret
}

func (this *SessionManager) GetSessionCount() int {
	this.mu.Lock()
	ret := len(this.sidMaps)
	this.mu.Unlock()
	return ret
}

func (this *SessionManager) RemoveSession(s *Session) {
	this.mu.Lock()
	delete(this.sidMaps, s.Sid)
	delete(this.uidMaps, s.Uid)
	this.mu.Unlock()
    s.closeMsgQ()
}

func (this *SessionManager) RemoveSessionBySid(sid string) *Session {
	this.mu.Lock()
	s, ok := this.sidMaps[sid]
	if ok {
		delete(this.sidMaps, sid)
		delete(this.uidMaps, s.Uid)
	}
	this.mu.Unlock()
	return s
}

func (this *SessionManager) RemoveSessionByUid(uid string) *Session {
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
        v.mu = sync.Mutex{}
		v.msgQ = make(chan *Message, 16)
		v.DownQueue = NewSafeQueue()
        v.isClosed = false
		this.uidMaps[v.Uid] = v
	}
	return l1, nil
}
