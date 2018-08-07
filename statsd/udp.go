package statsd

import (
	"errors"
	"net"
	"time"
)

// udpWriter is an internal class wrapping around management of UDP connection
type udpWriter struct {
	conn    net.Conn
	addr    string
	udpAddr *net.UDPAddr
	close   chan struct{}
	update  chan *net.UDPAddr
}

const dnsLookupPeriod = 30

// New returns a pointer to a new udpWriter given an addr in the format "hostname:port".
func newUDPWriter(addr string) (*udpWriter, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	writer := &udpWriter{
		conn:    conn,
		addr:    addr,
		udpAddr: udpAddr,
		close:   make(chan struct{}),
		update:  make(chan *net.UDPAddr),
	}
	go writer.detectDNSChange()
	return writer, nil
}

func (w *udpWriter) detectDNSChange() {
	ticker := time.NewTicker(dnsLookupPeriod * time.Second)
	for {
		select {
		case <-ticker.C:
			udpAddr, err := net.ResolveUDPAddr("udp", w.addr)
			if err != nil {
				continue
			}

			// Port should never change but checking for sake of rigor and completion
			if !udpAddr.IP.Equal(w.udpAddr.IP) || udpAddr.Port != w.udpAddr.Port {
				w.update <- udpAddr
			}
		case <-w.close:
			return
		}
	}
}

// SetWriteTimeout is not needed for UDP, returns error
func (w *udpWriter) SetWriteTimeout(d time.Duration) error {
	return errors.New("SetWriteTimeout: not supported for UDP connections")
}

// Write data to the UDP connection with no error handling
func (w *udpWriter) Write(data []byte) (int, error) {
	select {
	case udpAddr := <-w.update:
		conn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return 0, err
		}
		w.conn = conn
	default:
		// NOTHING
	}
	return w.conn.Write(data)
}

func (w *udpWriter) Close() error {
	close(w.update)
	close(w.close)
	return w.conn.Close()
}
