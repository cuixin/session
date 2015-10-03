package session

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

// 实现一个双向唯一Sid<->Uid
func NewSessionManager() *SessionManager {
	return &SessionManager{
		mu:        sync.Mutex{},
		closeChan: make(chan struct{}),
		closeWait: make(chan struct{}, 1),
		closeFlag: 0,
		sidMaps:   make(map[string]*Session, 16<<10), // 16384
		uidMaps:   make(map[string]*Session, 16<<10),
	}
}

type SessionManager struct {
	mu        sync.Mutex
	closeChan chan struct{} `msgpack:"-"`
	closeWait chan struct{} `msgpack:"-"`
	closeFlag int32

	sidMaps map[string]*Session
	uidMaps map[string]*Session
}

// 创建一个Session，如果存在则返回false，新创建的返回true
func (this *SessionManager) NewSession(sid, uid, remoteAddr string) (*Session, bool) {
	if sid == "" {
		return nil, false
	}
	nowTime := time.Now()
	s := &Session{
		Sid:            sid,
		Uid:            uid,
		RemoteAddr:     remoteAddr,
		ConnectTime:    nowTime,
		LastPacketTime: nowTime,
		LastIOTime:     nowTime,
		PacketCount:    1,
		DownQueue:      NewSafeQueue(),
	}

	this.mu.Lock()
	if oldSession, ok := this.uidMaps[uid]; ok {
		this.mu.Unlock()
		return oldSession, false
	}
	this.sidMaps[sid] = s
	this.uidMaps[uid] = s
	this.mu.Unlock()
	return s, true
}

// 克隆所有的Uid
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

// 克隆所有的Sid
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
		v.DownQueue = NewSafeQueue()
		this.uidMaps[v.Uid] = v
	}
	this.closeChan = make(chan struct{})
	this.closeWait = make(chan struct{}, 1)
	return l1, nil
}

// 开启回收Session的Goroutine
func (this *SessionManager) StartRecycleRoutine(period, timeout time.Duration, doSessionCheck, doSessionClose func(*Session)) {
	if doSessionCheck == nil && doSessionClose == nil {
		return
	}
	if atomic.LoadInt32(&this.closeFlag) == 0 {
		go func() {
		lout:
			for {
				select {
				case <-time.After(period):
					this.mu.Lock()
					for _, v := range this.sidMaps {
						// expired
						if time.Now().After(v.LastPacketTime.Add(timeout)) {
							delete(this.sidMaps, v.Sid)
							delete(this.uidMaps, v.Uid)
							if doSessionClose != nil {
								go doSessionClose(v)
							}
						} else {
							if doSessionCheck != nil {
								go doSessionCheck(v)
							}
						}
					}
					this.mu.Unlock()
				case <-this.closeChan:
					fmt.Println("close chan")
					break lout
				}
			}
			fmt.Println("Routine Closed")
			this.closeWait <- struct{}{}
		}()
	}
}

// 关闭所有Session
func (this *SessionManager) StopRecycleRoutine() {
	if atomic.CompareAndSwapInt32(&this.closeFlag, 0, 1) {
		this.closeChan <- struct{}{}
		<-this.closeWait
		close(this.closeChan)
		close(this.closeWait)
		fmt.Println("Close Sessions OK")
	}
}

// 立即检查是否可以回收Session
func (this *SessionManager) RecycleNow(timeout time.Duration, doSessionClose func(*Session)) {
	fmt.Println("Routine Recycle imediately")
	this.mu.Lock()
	now := time.Now()
	for _, v := range this.sidMaps {
		// expired
		if now.After(v.LastPacketTime.Add(timeout)) {
			// fmt.Println(v.Uid, "Expired")
			delete(this.sidMaps, v.Sid)
			delete(this.uidMaps, v.Uid)
			doSessionClose(v)
		}
	}
	this.mu.Unlock()
}
