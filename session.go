package session

import (
	"sync"
	"time"
)

// 用户会话
type Session struct {
	Sid            string      // 用户SessionId
	Uid            string      // 用户Uid
	RemoteAddr     string      // 远程连接地址
	ConnectTime    time.Time   // 连接时间
	LastPacketTime time.Time   // 最后一次发包时间，判定是否玩家已经超时离线
	LastIOTime     time.Time   // 最后一次IO写入的时间
	PacketCount    int64       // 发送请求包的总数量
	Attachment     interface{} `msgpack:"-"` // 绑定的数据
	DownQueue      *SafeQueue  `msgpack:"-"` // 下行数据的队列

	sync.Mutex // 内部消息处理锁，保证单线程处理，此部分由用户自行控制
}
