package forwarder

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"
)

type Forwarder struct {
	destinations []string
	Transparent  bool
	mu           sync.RWMutex
	rawFD        int
	rawOnce      sync.Once
	rawErr       error
}

func (f *Forwarder) AddDestination(destination string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destinations = append(f.destinations, destination)
}

func (f *Forwarder) initRawSocket() {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		f.rawErr = fmt.Errorf("failed to create raw socket: %w", err)
		return
	}
	err = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)
	if err != nil {
		syscall.Close(fd)
		f.rawErr = fmt.Errorf("failed to set IP_HDRINCL: %w", err)
		return
	}
	f.rawFD = fd
}

func (f *Forwarder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rawFD != 0 {
		err := syscall.Close(f.rawFD)
		f.rawFD = 0
		f.rawOnce = sync.Once{}
		f.rawErr = nil
		return err
	}
	return nil
}

func (f *Forwarder) StartForwarding(conn *net.UDPConn, packet []byte, addr *net.UDPAddr) {
	f.mu.RLock()
	destinations := make([]string, len(f.destinations))
	copy(destinations, f.destinations)
	transparent := f.Transparent
	f.mu.RUnlock()

	useRaw := false
	var fd int
	if transparent {
		if addr != nil && addr.IP.To4() != nil {
			f.rawOnce.Do(f.initRawSocket)
			if f.rawErr != nil {
				log.Printf("Warning: transparent mode is enabled but raw socket could not be initialized: %v. Falling back to standard forwarding.", f.rawErr)
			} else {
				useRaw = true
				fd = f.rawFD
			}
		}
	}

	for _, destination := range destinations {
		destAddr, err := net.ResolveUDPAddr("udp", destination)
		if err != nil {
			continue
		}

		if useRaw && destAddr.IP.To4() != nil {
			rawPacket, err := buildIPv4UDPPacket(addr.IP, destAddr.IP, addr.Port, destAddr.Port, packet)
			if err != nil {
				log.Printf("Error building raw UDP packet: %v. Falling back to standard forwarding.", err)
				_, _ = conn.WriteToUDP(packet, destAddr)
				continue
			}

			toAddr := &syscall.SockaddrInet4{
				Port: destAddr.Port,
			}
			copy(toAddr.Addr[:], destAddr.IP.To4())

			err = syscall.Sendto(fd, rawPacket, 0, toAddr)
			if err != nil {
				log.Printf("Error sending raw UDP packet: %v. Falling back to standard forwarding.", err)
				_, _ = conn.WriteToUDP(packet, destAddr)
			}
		} else {
			_, err = conn.WriteToUDP(packet, destAddr)
			if err != nil {
				continue
			}
		}
	}
}

func buildIPv4UDPPacket(srcIP, dstIP net.IP, srcPort, dstPort int, payload []byte) ([]byte, error) {
	srcIP4 := srcIP.To4()
	dstIP4 := dstIP.To4()
	if srcIP4 == nil || dstIP4 == nil {
		return nil, fmt.Errorf("both source and destination must be IPv4")
	}

	totalLen := 20 + 8 + len(payload)
	if totalLen > 65535 {
		return nil, fmt.Errorf("payload too large")
	}

	packet := make([]byte, totalLen)

	// Version (4) & IHL (5) -> 0x45
	packet[0] = 0x45
	// TOS -> 0
	packet[1] = 0x00
	// Total Length
	binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
	// Identification -> 0
	binary.BigEndian.PutUint16(packet[4:6], 0)
	// Flags & Fragment Offset (Don't Fragment = 0x4000)
	binary.BigEndian.PutUint16(packet[6:8], 0x4000)
	// TTL -> 64
	packet[8] = 64
	// Protocol -> 17 (UDP)
	packet[9] = 17
	// Header Checksum -> 0 (for now)
	binary.BigEndian.PutUint16(packet[10:12], 0)
	// Source IP
	copy(packet[12:16], srcIP4)
	// Destination IP
	copy(packet[16:20], dstIP4)

	// Calculate IP checksum
	ipChecksum := checksum(packet[0:20])
	binary.BigEndian.PutUint16(packet[10:12], ipChecksum)

	// 2. UDP Header
	// Source Port
	binary.BigEndian.PutUint16(packet[20:22], uint16(srcPort))
	// Destination Port
	binary.BigEndian.PutUint16(packet[22:24], uint16(dstPort))
	// UDP Length
	udpLen := 8 + len(payload)
	binary.BigEndian.PutUint16(packet[24:26], uint16(udpLen))
	// UDP Checksum -> 0 (for now)
	binary.BigEndian.PutUint16(packet[26:28], 0)

	// Copy payload
	copy(packet[28:], payload)

	// Calculate UDP checksum using pseudo-header
	pseudoHeader := make([]byte, 12+udpLen)
	copy(pseudoHeader[0:4], srcIP4)
	copy(pseudoHeader[4:8], dstIP4)
	pseudoHeader[8] = 0
	pseudoHeader[9] = 17
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(udpLen))
	copy(pseudoHeader[12:], packet[20:])

	udpChecksum := checksum(pseudoHeader)
	if udpChecksum == 0 {
		udpChecksum = 0xffff
	}
	binary.BigEndian.PutUint16(packet[26:28], udpChecksum)

	return packet, nil
}

func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data)-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i : i+2]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}