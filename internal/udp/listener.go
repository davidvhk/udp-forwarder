package udp

import (
	"log"
	"net"

	"github.com/davidvhk/udp-forwarder/internal/forwarder"
)

type Listener struct {
	Address   string
	Forwarder *forwarder.Forwarder
}

func (l *Listener) StartListening(address string, forwarder *forwarder.Forwarder) error {
	l.Address = address
	l.Forwarder = forwarder

	addr, err := net.ResolveUDPAddr("udp", l.Address)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Printf("Listening for UDP traffic on %s", l.Address)

	buffer := make([]byte, 1024)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		log.Printf("Received %d bytes from %s", n, remoteAddr.String())
		go l.Forwarder.StartForwarding(conn, buffer[:n], remoteAddr)
	}
}
