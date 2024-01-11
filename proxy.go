package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	LenBufSize  = 2
	TunMTU      = 65535 // The maximum-allowed MTU for the tun interface.
	TunName     = "tun0"
	DefaultPort = 1024
)

var l = log.New(
	os.Stderr,
	"nitriding-proxy: ",
	log.Ldate|log.Ltime|log.LUTC|log.Lshortfile,
)

// TunToVsock forwards network packets from the tun device to our
// TCP-over-VSOCK connection. The function keeps on forwarding packets until we
// encounter an error or EOF. Errors (including EOF) are written to the given
// channel.
func TunToVsock(from io.Reader, to io.WriteCloser, ch chan error) {
	defer to.Close()
	var (
		err       error
		pktLenBuf = make([]byte, LenBufSize)
		pktBuf    = make([]byte, TunMTU)
	)

	for {
		// Read a network packet from the tun interface.
		nr, rerr := from.Read(pktBuf)
		if nr > 0 {
			// Forward the network packet to our TCP-over-VSOCK connection.
			binary.BigEndian.PutUint16(pktLenBuf, uint16(nr))
			if _, werr := to.Write(append(pktLenBuf, pktBuf[:nr]...)); err != nil {
				err = werr
				break
			}
		}
		if rerr != nil {
			err = rerr
			break
		}
	}
	ch <- fmt.Errorf("stopped tun-to-vsock forwarding because: %v", err)
}

// VsockToTun forwards network packets from our TCP-over-VSOCK connection to
// the tun interface. The function keeps on forwarding packets until we
// encounter an error or EOF. Errors (including EOF) are written to the given
// channel.
func VsockToTun(from io.Reader, to io.Writer, ch chan error) {
	var (
		err       error
		pktLen    uint16
		pktLenBuf = make([]byte, LenBufSize)
		pktBuf    = make([]byte, TunMTU)
	)

	for {
		// Read the length prefix that tells us the size of the subsequent
		// packet.
		if _, err = io.ReadFull(from, pktLenBuf); err != nil {
			break
		}
		pktLen = binary.BigEndian.Uint16(pktLenBuf)

		// Read the packet.
		if _, err = io.ReadFull(from, pktBuf[:pktLen]); err != nil {
			break
		}

		// Forward the packet to the tun interface.
		if _, err := to.Write(pktBuf[:pktLen]); err != nil {
			break
		}
	}
	ch <- fmt.Errorf("stopped vsock-to-tun forwarding because: %v", err)
}
