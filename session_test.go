package session

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"os"
	"testing"
	"time"
)

var (
	sm *SessionManager = NewSessionManager()
	s  *Session
)

func TestNewSessionId(t *testing.T) {
	sid := NewSessionId(32)
	if sid == "" {
		t.Fatal("Wrong NewSessionId")
	}
}

func BenchmarkNewSessionId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewSessionId(32)
	}
}

func TestNewSession(t *testing.T) {
	var ok bool
	s, ok = sm.NewSession("1111", "2222", "192.168.1.1")
	if s == nil || !ok {
		t.Error("Check NewSession error")
	}
}

func TestCheckSession(t *testing.T) {
	s = sm.GetSessionBySid("1111")
	if s == nil {
		t.Error("Check GetSession error")
	}
}

func TestCheckSessionCount(t *testing.T) {
	c := sm.GetSessionCount()
	if c != 1 {
		t.Error("Check count error")
	}
}

func TestRemoveSessionBySid(t *testing.T) {
	c := sm.RemoveSessionBySid("1111")
	if c == nil {
		t.Failed()
	}
	if sm.GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestRemoveSessionByUid(t *testing.T) {
	sm.NewSession("1111", "2222", "192.168.1.23")

	c := sm.RemoveSessionByUid("2222")
	if c == nil {
		t.Failed()
	}
	if sm.GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestRemoveSession(t *testing.T) {
	x, ok := sm.NewSession("1111", "2222", "192.168.1.23")
	if !ok {
		t.Error("Error on new session")
	}
	sm.RemoveSession(x)

	if sm.GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestSessionQueue(t *testing.T) {
	s, _ := sm.NewSession("1", "101", "193.168.1.1")
	s2, _ := sm.NewSession("2", "102", "193.168.1.2")

	x := sm.GetSessionBySid("1")
	fmt.Printf("%p xxx \n", x)

	if x == nil {
		t.Fatal("Nil session")
	}
	x.PushQueue("111")
	x.PushQueue("222")
	x.PushQueue("333")
	y := sm.GetSessionByUid("101")
	fmt.Printf("%p yyy \n", y)
	if y == nil {
		t.Fatal("Nil session")
	}
	ret := y.RemoveQueue()
	if len(ret) != 3 {
		t.Fatal("error queue count")
	}
	fmt.Println(ret)
	sm.RemoveSession(s)
	sm.RemoveSession(s2)
}

func TestSessionManDumpToFile(t *testing.T) {
	sm.NewSession("1", "101", "193.168.1.1")
	sm.NewSession("2", "102", "193.168.1.2")
	sm.NewSession("3", "103", "193.168.1.3")
	sm.NewSession("4", "104", "193.168.1.4")
	sm.NewSession("5", "105", "193.168.1.5")
	err := sm.DumpToFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
}

func TestSessionManLoadFromFile(t *testing.T) {
	_, err := sm.LoadFromFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("Sids:")
	for k, v := range sm.sidMaps {
		fmt.Println(k, v)
	}
	fmt.Println("Uids:")
	for k, v := range sm.uidMaps {
		fmt.Println(k, v)
	}
	s5 := sm.GetSessionBySid("4")
	s5.PushQueue("1")

	s55 := sm.GetSessionByUid("104")
	ret := s55.RemoveQueue()
	if len(ret) != 1 {
		t.Fatal("error queue")
	}
	os.Remove("session.db")
}

func TestGetSessionUids(t *testing.T) {
	sm.ClearSession()
	if sm.GetSessionCount() != 0 {
		t.Fatal("Session count not 0")
		return
	}
	sm.NewSession("1", "101", "193.168.1.1")
	sm.NewSession("2", "102", "193.168.1.2")
	sm.NewSession("3", "103", "193.168.1.3")
	sm.NewSession("4", "104", "193.168.1.4")
	sm.NewSession("5", "105", "193.168.1.5")
	uids := sm.GetAllSessionUids()
	fmt.Println(uids)
	if len(uids) != 5 {
		t.Fatal("Session count not 5")
	}
}

func TestSessionRecycle(t *testing.T) {
	sm.StartRecycle(1*time.Second, time.Second*4)
	time.Sleep(5 * time.Second)
	sm.NewSession("1", "101", "193.168.1.1")
	if sm.GetSessionCount() != 1 {
		t.Fatal("error")
	}
	time.Sleep(5 * time.Second)

	if sm.GetSessionCount() != 0 {
		t.Fatal("error count")
	}
}

func BenchmarkTestSessionId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewSessionId(32)
	}
}

func BenchmarkTestUUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		uuid.New()
	}
}
