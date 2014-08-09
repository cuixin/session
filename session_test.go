package session

import (
	"fmt"
	"github.com/cuixin/goalg/queue"
	"os"
	"sync"
	"testing"
)

var s *Session

func TestNewSession(t *testing.T) {
	var ok bool
	s, ok = NewSession("1111", "2222", "192.168.1.1")
	if s == nil || !ok {
		t.Error("Check NewSession error")
	}
}

func TestCheckSession(t *testing.T) {
	s = GetSessionBySid("1111")
	if s == nil {
		t.Error("Check GetSession error")
	}
}

func TestCheckSessionCount(t *testing.T) {
	c := GetSessionCount()
	if c != 1 {
		t.Error("Check count error")
	}
}

func TestRemoveSessionBySid(t *testing.T) {
	c := RemoveSessionBySid("1111")
	if c == nil {
		t.Failed()
	}
	if GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestRemoveSessionByUid(t *testing.T) {
	NewSession("1111", "2222", "192.168.1.23")

	c := RemoveSessionByUid("2222")
	if c == nil {
		t.Failed()
	}
	if GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestRemoveSession(t *testing.T) {
	x, ok := NewSession("1111", "2222", "192.168.1.23")
	if !ok {
		t.Error("Error on new session")
	}
	RemoveSession(x)

	if GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestSessionQueue(t *testing.T) {
	s := &Session{
		Sid:        "1112",
		Uid:        "2233",
		writeLock:  &sync.Mutex{},
		writeQueue: queue.New()}
	s.PushQueue("111")
	s.PushQueue("222")
	s.PushQueue("333")
	retQueue := s.RemoveQueue()
	retQueue = append(retQueue, "444")

	fmt.Println(retQueue)
}

func TestSessionManDumpToFile(t *testing.T) {
	NewSession("1", "101", "193.168.1.1")
	NewSession("2", "102", "193.168.1.2")
	NewSession("3", "103", "193.168.1.3")
	NewSession("4", "104", "193.168.1.4")
	NewSession("5", "105", "193.168.1.5")
	err := DumpToFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
}

func TestSessionManLoadFromFile(t *testing.T) {
	_, err := LoadFromFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("Sids:")
	for k, v := range this.sidMaps {
		fmt.Println(k, v)
	}
	fmt.Println("Uids:")
	for k, v := range this.uidMaps {
		fmt.Println(k, v)
	}
	os.Remove("session.db")
}
