package forwarder

import (
	"net"
	"sync"
)

type Forwarder struct {
	destinations []string
	mu           sync.RWMutex
}

func (f *Forwarder) AddDestination(destination string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destinations = append(f.destinations, destination)
}

func (f *Forwarder) StartForwarding(conn *net.UDPConn, packet []byte, addr *net.UDPAddr) {
	f.mu.RLock()
	destinations := make([]string, len(f.destinations))
	copy(destinations, f.destinations)
	f.mu.RUnlock()

	for _, destination := range destinations {
		destAddr, err := net.ResolveUDPAddr("udp", destination)
		if err != nil {
			continue
		}
		_, err = conn.WriteToUDP(packet, destAddr)
		if err != nil {
			continue
		}
	}
}