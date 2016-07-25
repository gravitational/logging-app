package trace

import (
	"encoding/json"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/jonboulle/clockwork"
)

const (
	// UDPDefaultAddr is a default address to emit logs to
	UDPDefaultAddr = "127.0.0.1:5000"
	// UDPDefaultNet is a default network
	UDPDefaultNet = "udp"
)

// UDPOptionSetter represents functional arguments passed to ELKHook
type UDPOptionSetter func(f *UDPHook)

// NewUDPHook returns logrus-compatible hook that sends data to UDP socket
func NewUDPHook(opts ...UDPOptionSetter) (*UDPHook, error) {
	f := &UDPHook{}
	for _, o := range opts {
		o(f)
	}
	if f.Clock == nil {
		f.Clock = clockwork.NewRealClock()
	}
	if f.clientNet == "" {
		f.clientNet = UDPDefaultNet
	}
	if f.clientAddr == "" {
		f.clientAddr = UDPDefaultAddr
	}
	addr, err := net.ResolveUDPAddr(f.clientNet, f.clientAddr)
	if err != nil {
		return nil, Wrap(err)
	}
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, Wrap(err)
	}
	f.addr = addr
	f.conn = conn.(*net.UDPConn)
	return f, nil
}

type UDPHook struct {
	Clock      clockwork.Clock
	clientNet  string
	clientAddr string
	addr       *net.UDPAddr
	conn       *net.UDPConn
}

type Frame struct {
	Time    time.Time              `json:"time"`
	Type    string                 `json:"type"`
	Entry   map[string]interface{} `json:"entry"`
	Message string                 `json:"message"`
	Level   string                 `json:"level"`
}

// Fire fires the event to the ELK beat
func (elk *UDPHook) Fire(e *log.Entry) error {
	if frameNo := findFrame(); frameNo != -1 {
		t := newTrace(frameNo-1, nil)
		e.Data[FileField] = t.String()
		e.Data[FunctionField] = t.Func()
	}
	data, err := json.Marshal(Frame{
		Time:    elk.Clock.Now().UTC(),
		Type:    "trace",
		Entry:   e.Data,
		Message: e.Message,
		Level:   e.Level.String(),
	})
	if err != nil {
		return Wrap(err)
	}

	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return Wrap(err)
	}

	ra, err := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	if err != nil {
		return Wrap(err)
	}

	_, err = (c.(*net.UDPConn)).WriteToUDP(data, ra)
	return Wrap(err)

}

// Levels returns logging levels supported by logrus
func (elk *UDPHook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
		log.DebugLevel,
	}
}
