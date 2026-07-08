package forwarder

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
)

type Forwarder struct {
	destinations []string
	Transparent  bool
	MTU          int
	mu           sync.RWMutex
	rawFD        int
	rawOnce      sync.Once
	rawErr       error
}

var ipID uint32

func (f *Forwarder) AddDestination(destination string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destinations = append(f.destinations, destination)
}

const (
	ipMTUDiscover  = 10
	ipPMTUDiscDont = 0
)

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
	// Disable PMTUD to prevent EMSGSIZE errors on packets exceeding path MTU
	// by allowing the kernel to perform IP fragmentation.
	err = syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, ipMTUDiscover, ipPMTUDiscDont)
	if err != nil {
		log.Printf("Warning: failed to set IP_MTU_DISCOVER on raw socket: %v", err)
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
	mtu := f.MTU
	if mtu <= 0 {
		mtu = 1500
	}
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
			id := uint16(atomic.AddUint32(&ipID, 1) & 0xffff)
			rawPackets, err := buildIPv4UDPFragments(addr.IP, destAddr.IP, addr.Port, destAddr.Port, packet, mtu, id)
			if err != nil {
				log.Printf("Error building raw UDP packet fragments (src=%s, dst=%s, len=%d, mtu=%d): %v. Falling back to standard forwarding.", addr.IP, destAddr.IP, len(packet), mtu, err)
				_, _ = conn.WriteToUDP(packet, destAddr)
				continue
			}

			toAddr := &syscall.SockaddrInet4{
				Port: destAddr.Port,
			}
			copy(toAddr.Addr[:], destAddr.IP.To4())

			if len(rawPackets) > 1 {
				log.Printf("Fragmented packet of %d bytes into %d fragments for transparent forwarding (mtu=%d)", len(packet), len(rawPackets), mtu)
			}

			sendErr := false
			for i, frag := range rawPackets {
				err = syscall.Sendto(fd, frag, 0, toAddr)
				if err != nil {
					log.Printf("Error sending raw UDP packet fragment %d/%d (fragLen=%d, totalLen=%d, destination=%s): %v. Falling back to standard forwarding.", i+1, len(rawPackets), len(frag), len(packet), destination, err)
					sendErr = true
					break
				}
			}
			if sendErr {
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
	frags, err := buildIPv4UDPFragments(srcIP, dstIP, srcPort, dstPort, payload, 65535, 0)
	if err != nil {
		return nil, err
	}
	return frags[0], nil
}

func buildIPv4UDPFragments(srcIP, dstIP net.IP, srcPort, dstPort int, payload []byte, mtu int, id uint16) ([][]byte, error) {
	srcIP4 := srcIP.To4()
	dstIP4 := dstIP.To4()
	if srcIP4 == nil || dstIP4 == nil {
		return nil, fmt.Errorf("both source and destination must be IPv4")
	}

	// Calculate UDP length and build the full unfragmented UDP datagram (UDP header + payload)
	// so we can calculate the correct UDP checksum.
	udpLen := 8 + len(payload)
	if udpLen > 65535 {
		return nil, fmt.Errorf("payload too large")
	}

	udpPacket := make([]byte, udpLen)
	binary.BigEndian.PutUint16(udpPacket[0:2], uint16(srcPort))
	binary.BigEndian.PutUint16(udpPacket[2:4], uint16(dstPort))
	binary.BigEndian.PutUint16(udpPacket[4:6], uint16(udpLen))
	binary.BigEndian.PutUint16(udpPacket[6:8], 0)
	copy(udpPacket[8:], payload)

	// Calculate UDP checksum using pseudo-header
	pseudoHeader := make([]byte, 12+udpLen)
	copy(pseudoHeader[0:4], srcIP4)
	copy(pseudoHeader[4:8], dstIP4)
	pseudoHeader[8] = 0
	pseudoHeader[9] = 17
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(udpLen))
	copy(pseudoHeader[12:], udpPacket)

	udpChecksum := checksum(pseudoHeader)
	if udpChecksum == 0 {
		udpChecksum = 0xffff
	}
	binary.BigEndian.PutUint16(udpPacket[6:8], udpChecksum)

	// Determine if we need to fragment
	totalLen := 20 + udpLen
	if totalLen <= mtu {
		// No fragmentation needed
		packet := make([]byte, totalLen)

		// IP Header
		packet[0] = 0x45
		packet[1] = 0x00
		binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
		binary.BigEndian.PutUint16(packet[4:6], id)
		binary.BigEndian.PutUint16(packet[6:8], 0x0000) // Clear DF
		packet[8] = 64
		packet[9] = 17
		copy(packet[12:16], srcIP4)
		copy(packet[16:20], dstIP4)

		ipChecksum := checksum(packet[0:20])
		binary.BigEndian.PutUint16(packet[10:12], ipChecksum)

		// UDP Header + Payload
		copy(packet[20:], udpPacket)
		return [][]byte{packet}, nil
	}

	// Fragment payload
	maxFragPayload := (mtu - 20) / 8 * 8
	if maxFragPayload <= 8 {
		return nil, fmt.Errorf("MTU %d is too small for raw IP/UDP forwarding", mtu)
	}

	var fragments [][]byte
	offsetBytes := 0

	for offsetBytes < udpLen {
		chunkSize := udpLen - offsetBytes
		if chunkSize > maxFragPayload {
			chunkSize = maxFragPayload
		}

		fragTotalLen := 20 + chunkSize
		packet := make([]byte, fragTotalLen)

		// IP Header
		packet[0] = 0x45
		packet[1] = 0x00
		binary.BigEndian.PutUint16(packet[2:4], uint16(fragTotalLen))
		binary.BigEndian.PutUint16(packet[4:6], id)

		// Flags & Fragment Offset
		var flagsAndOffset uint16 = uint16(offsetBytes / 8)
		if offsetBytes+chunkSize < udpLen {
			flagsAndOffset |= 0x2000 // MF (More Fragments) flag
		}
		binary.BigEndian.PutUint16(packet[6:8], flagsAndOffset)

		packet[8] = 64
		packet[9] = 17
		copy(packet[12:16], srcIP4)
		copy(packet[16:20], dstIP4)

		ipChecksum := checksum(packet[0:20])
		binary.BigEndian.PutUint16(packet[10:12], ipChecksum)

		// Payload chunk
		copy(packet[20:], udpPacket[offsetBytes:offsetBytes+chunkSize])

		fragments = append(fragments, packet)
		offsetBytes += chunkSize
	}

	return fragments, nil
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