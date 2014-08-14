package session

import (
	"fmt"
	"os"
	"testing"
	"time"
)

var s *Session

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
	s, _ := NewSession("1", "101", "193.168.1.1")
	s2, _ := NewSession("2", "102", "193.168.1.2")

	x := GetSessionBySid("1")
	fmt.Printf("%p xxx \n", x)

	if x == nil {
		t.Fatal("Nil session")
	}
	x.PushQueue("111")
	x.PushQueue("222")
	x.PushQueue("333")
	y := GetSessionByUid("101")
	fmt.Printf("%p yyy \n", y)
	if y == nil {
		t.Fatal("Nil session")
	}
	ret := y.RemoveQueue()
	if len(ret) != 3 {
		t.Fatal("error queue count")
	}
	fmt.Println(ret)
	RemoveSession(s)
	RemoveSession(s2)
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
	s5 := GetSessionBySid("4")
	s5.PushQueue("1")

	s55 := GetSessionByUid("104")
	ret := s55.RemoveQueue()
	if len(ret) != 1 {
		t.Fatal("error queue")
	}
	os.Remove("session.db")
}

func TestSessionRecycle(t *testing.T) {
	StartRecycle(1*time.Second, time.Second*4)
	time.Sleep(5 * time.Second)
	NewSession("1", "101", "193.168.1.1")
	if GetSessionCount() != 1 {
		t.Fatal("error")
	}
	time.Sleep(5 * time.Second)

	if GetSessionCount() != 0 {
		t.Fatal("error count")
	}
}
