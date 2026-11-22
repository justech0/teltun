package droid

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

// handler gets every TCP connection and UDP flow from the VPN and sends it
// to the phone as a SOCKS5 session inside the yamux link (UDP as sing's
// UDP-over-TCP, which the phone helper understands).
type handler struct {
	e         *Engine
	blockQUIC bool
}

var errBlocked = errors.New("blocked")

func (h *handler) PrepareConnection(network string, _ M.Socksaddr, dst M.Socksaddr,
	_ tun.DirectRouteContext, _ time.Duration) (tun.DirectRouteDestination, error) {
	// QUIC over a TCP link is slow; browsers fall back to TCP.
	if h.blockQUIC && network == N.NetworkUDP && dst.Port == 443 {
		return nil, errBlocked
	}
	return nil, nil
}

func (h *handler) NewConnectionEx(_ context.Context, conn net.Conn, _ M.Socksaddr, dst M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		err := h.tcp(conn, dst)
		conn.Close()
		if onClose != nil {
			onClose(err)
		}
	}()
}

func (h *handler) tcp(conn net.Conn, dst M.Socksaddr) error {
	s, err := h.e.openStream()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := socksConnect(s, dst); err != nil {
		return err
	}
	h.e.streams.Add(1)
	defer h.e.streams.Add(-1)
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(counter{s, &h.e.up}, conn)
		if cw, ok := s.(interface{ CloseWrite() error }); ok {
			cw.CloseWrite()
		} else {
			s.Close()
		}
		done <- struct{}{}
	}()
	go func() {
		io.Copy(counter{conn, &h.e.down}, s)
		conn.Close()
		done <- struct{}{}
	}()
	<-done
	<-done
	return nil
}

func (h *handler) NewPacketConnectionEx(_ context.Context, conn N.PacketConn, _ M.Socksaddr, dst M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		err := h.udp(conn, dst)
		conn.Close()
		if onClose != nil {
			onClose(err)
		}
	}()
}

func (h *handler) udp(conn N.PacketConn, dst M.Socksaddr) error {
	s, err := h.e.openStream()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := socksConnect(s, M.Socksaddr{Fqdn: uot.MagicAddress}); err != nil {
		return err
	}
	req := uot.Request{IsConnect: false, Destination: dst}
	if err := uot.WriteRequest(s, req); err != nil {
		return err
	}
	uc := uot.NewConn(s, req)

	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			b := buf.NewPacket()
			to, err := conn.ReadPacket(b)
			if err != nil {
				b.Release()
				return
			}
			h.e.up.Add(uint64(b.Len()))
			err = uc.WritePacket(b, to)
			b.Release()
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			b := buf.NewPacket()
			from, err := uc.ReadPacket(b)
			if err != nil {
				b.Release()
				return
			}
			h.e.down.Add(uint64(b.Len()))
			if err := conn.WritePacket(b, from); err != nil {
				return
			}
		}
	}()
	<-done
	s.Close()
	conn.Close()
	<-done
	return nil
}

// socksConnect does a no-auth SOCKS5 CONNECT on an already open stream.
func socksConnect(s net.Conn, dst M.Socksaddr) error {
	req := []byte{5, 1, 0, 5, 1, 0}
	if dst.IsFqdn() {
		req = append(req, 3, byte(len(dst.Fqdn)))
		req = append(req, dst.Fqdn...)
	} else if dst.Addr.Is4() {
		a := dst.Addr.As4()
		req = append(req, 1)
		req = append(req, a[:]...)
	} else {
		a := dst.Addr.As16()
		req = append(req, 4)
		req = append(req, a[:]...)
	}
	req = binary.BigEndian.AppendUint16(req, dst.Port)
	if _, err := s.Write(req); err != nil {
		return err
	}
	resp := make([]byte, 2+10) // method reply + CONNECT reply (IPv4 bound address)
	if _, err := io.ReadFull(s, resp); err != nil {
		return err
	}
	if resp[3] != 0 {
		return errors.New("phone could not connect")
	}
	return nil
}
