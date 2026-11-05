package tcptls

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/server/core"
	"github.com/digitizables/lattice/server/listen"
)

// Channel accepts TLS TCP connections and speaks length-prefixed Protobuf frames.
// Frame: 1 byte msg type + 4 byte big-endian length + protobuf body.
// Types: 1=checkin, 2=checkin_resp, 3=result, 4=result_resp.
type Channel struct {
	core     *core.Core
	log      *slog.Logger
	certFile string
	keyFile  string
	useTLS   bool

	mu       sync.Mutex
	listener net.Listener
}

// New builds a TCP channel.
func New(c *core.Core, log *slog.Logger) *Channel {
	if log == nil {
		log = slog.Default()
	}
	return &Channel{core: c, log: log}
}

// WithTLS enables TLS for the listener.
func (c *Channel) WithTLS(certFile, keyFile string) *Channel {
	c.certFile = certFile
	c.keyFile = keyFile
	c.useTLS = true
	return c
}

func (c *Channel) Name() string { return "tcp-tls" }

func (c *Channel) Start(ctx context.Context, addr string) error {
	var ln net.Listener
	var err error
	if c.useTLS {
		cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
		if err != nil {
			return err
		}
		cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		ln, err = tls.Listen("tcp", addr, cfg)
		if err != nil {
			return err
		}
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.listener = ln
	c.mu.Unlock()

	go func() {
		c.log.Info("tcp-tls channel listening", "addr", addr, "tls", c.useTLS)
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					c.log.Debug("accept", "err", err)
					return
				}
			}
			go c.serve(conn)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = c.Stop(context.Background())
	}()
	return nil
}

func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	ln := c.listener
	c.listener = nil
	c.mu.Unlock()
	if ln != nil {
		return ln.Close()
	}
	return nil
}

const (
	msgCheckIn     = 1
	msgCheckInResp = 2
	msgResult      = 3
	msgResultResp  = 4
)

func (c *Channel) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Minute))
	for {
		typ, body, err := readFrame(conn)
		if err != nil {
			return
		}
		_ = conn.SetDeadline(time.Now().Add(2 * time.Minute))
		switch typ {
		case msgCheckIn:
			var in agentv1.AgentCheckIn
			if err := proto.Unmarshal(body, &in); err != nil {
				return
			}
			if c.core.RejectLocalAgent(conn.RemoteAddr().String(), in.GetHostname()) {
				return
			}
			out, err := c.core.HandleCheckIn(context.Background(), &in)
			if err != nil {
				c.log.Debug("tcp check-in failed", "err", err)
				return
			}
			raw, err := proto.Marshal(out)
			if err != nil {
				return
			}
			if err := writeFrame(conn, msgCheckInResp, raw); err != nil {
				return
			}
		case msgResult:
			var in agentv1.AgentTaskResult
			if err := proto.Unmarshal(body, &in); err != nil {
				return
			}
			out, err := c.core.HandleTaskResult(context.Background(), &in)
			if err != nil {
				c.log.Debug("tcp result failed", "err", err)
				return
			}
			raw, err := proto.Marshal(out)
			if err != nil {
				return
			}
			if err := writeFrame(conn, msgResultResp, raw); err != nil {
				return
			}
		default:
			return
		}
	}
}

func readFrame(r io.Reader) (byte, []byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	typ := hdr[0]
	n := binary.BigEndian.Uint32(hdr[1:])
	if n > 32<<20 {
		return 0, nil, fmt.Errorf("frame too large")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return typ, body, nil
}

func writeFrame(w io.Writer, typ byte, body []byte) error {
	hdr := make([]byte, 5)
	hdr[0] = typ
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

var _ listen.Channel = (*Channel)(nil)
