package session

import (
	"fmt"
	"os"
	"testing"
	"time"
)

var (
	sm   *SessionManager = NewSessionManager()
	uids                 = []string{"1", "2", "3", "4", "5"}
)

func TestNewSessionId(t *testing.T) {
	sid := newSessionId(sessionIdLength)
	if sid == "" {
		t.Fatal("Wrong NewSessionId")
	}
}

func BenchmarkNewSessionId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newSessionId(sessionIdLength)
	}
}

func testSessionLifecycle(uid string, t *testing.T) {
	s, ok := sm.NewSession(uid, "192.168.1.1")
	if s == nil || !ok {
		t.Error("Check NewSession error")
	}
	s = sm.GetSessionByUid(uid)
	if s == nil {
		t.Error("Check GetSession by Uid error")
	}
	s = sm.GetSessionBySid(s.Sid)
	if s == nil {
		t.Error("Check GetSession by Sid error")
	}
	c := sm.GetSessionCount()
	if c != 1 {
		t.Error("Check count error")
	}
	sm.RemoveSessionBySid(s.Sid)
	if sm.GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
	if sm.GetSessionBySid(s.Sid) != nil ||
		sm.GetSessionByUid(s.Uid) != nil {
		t.Error("Check Session is existed")
	}
	s, ok = sm.NewSession(uid, "192.168.1.1")
	if s == nil || !ok {
		t.Error("Check NewSession error")
	}
	c = sm.GetSessionCount()
	if c != 1 {
		t.Error("Check count error")
	}
	sm.RemoveSessionByUid(s.Uid)
	if sm.GetSessionCount() != 0 {
		t.Error("Cannot removed session")
	}
}

func TestSessionLifecycle(t *testing.T) {
	testSessionLifecycle("1111", t)
	testSessionLifecycle("2222", t)
	testSessionLifecycle("3333", t)
}

func TestSessionsDumpToFile(t *testing.T) {
	for _, s := range uids {
		sm.NewSession(s, "192.168.1.1")
	}
	err := sm.DumpToFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
}

func findUid(uid string) bool {
	for _, v := range uids {
		if uid == v {
			return true
		}
	}
	return false
}

func TestSessionsLoadFromFile(t *testing.T) {
	_, err := sm.LoadFromFile("session.db")
	if err != nil {
		t.Error(err)
		return
	}
	for _, v := range sm.sidMaps {
		if !findUid(v.Uid) {
			t.Fatal("Error find session")
		}
	}
	for uid, _ := range sm.uidMaps {
		if !findUid(uid) {
			t.Fatal("Error find session")
		}
	}
	os.Remove("session.db")
}

func TestSessionRecycle(t *testing.T) {
	checkFunc := func(s *Session) {
		fmt.Println(s.Uid, "Session Check")
	}
	closeFunc := func(s *Session) {
		fmt.Println(s.Uid, "Session Recycled")
	}
	sm.StartRecycleRoutine(time.Second, time.Second*2, checkFunc, closeFunc)
	time.Sleep(3 * time.Second)
	if sm.GetSessionCount() != 0 {
		t.Fatal("error")
	}
	for _, s := range uids {
		sm.NewSession(s, "192.168.1.1")
	}
	fmt.Println("Stop")
	time.Sleep(time.Second)
	sm.StopRecycleRoutine()
	time.Sleep(time.Second)
	sm.RecycleNow(time.Second*2, closeFunc)
	if sm.GetSessionCount() != 0 {
		t.Fatal("error")
	}
}
