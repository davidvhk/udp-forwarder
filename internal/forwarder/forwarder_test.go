package forwarder

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

func TestForwarder_StartForwarding(t *testing.T) {
	// Set up a mock destination UDP listener on local loopback
	destAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve UDP addr: %v", err)
	}

	destConn, err := net.ListenUDP("udp", destAddr)
	if err != nil {
		t.Fatalf("Failed to start mock destination listener: %v", err)
	}
	defer destConn.Close()

	resolvedDestAddr := destConn.LocalAddr().String()

	// Set up forwarder UDP connection
	fwdAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve forwarder addr: %v", err)
	}
	fwdConn, err := net.ListenUDP("udp", fwdAddr)
	if err != nil {
		t.Fatalf("Failed to start forwarder UDP conn: %v", err)
	}
	defer fwdConn.Close()

	f := &Forwarder{}
	f.AddDestination(resolvedDestAddr)

	testPayload := []byte("hello sflow data")

	// Start reading on the destination connection in a separate goroutine
	receivedChan := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 1024)
		n, _, err := destConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		receivedChan <- buf[:n]
	}()

	// Send packet
	f.StartForwarding(fwdConn, testPayload, nil)

	// Wait and verify
	select {
	case received := <-receivedChan:
		if string(received) != string(testPayload) {
			t.Errorf("Expected received payload %q, got %q", string(testPayload), string(received))
		}
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for forwarded packet")
	}
}

func TestForwarder_Concurrency(t *testing.T) {
	destAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve UDP addr: %v", err)
	}

	destConn, err := net.ListenUDP("udp", destAddr)
	if err != nil {
		t.Fatalf("Failed to start mock destination listener: %v", err)
	}
	defer destConn.Close()

	resolvedDestAddr := destConn.LocalAddr().String()

	fwdAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve forwarder addr: %v", err)
	}
	fwdConn, err := net.ListenUDP("udp", fwdAddr)
	if err != nil {
		t.Fatalf("Failed to start forwarder UDP conn: %v", err)
	}
	defer fwdConn.Close()

	f := &Forwarder{}
	f.AddDestination(resolvedDestAddr)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Launch multiple goroutines calling StartForwarding concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			f.StartForwarding(fwdConn, []byte{byte(id)}, nil)
		}(i)
	}

	// Just check for races during execution
	wg.Wait()
}

func TestForwarder_BuildIPv4UDPPacket(t *testing.T) {
	srcIP := net.ParseIP("192.168.1.50")
	dstIP := net.ParseIP("10.0.0.1")
	srcPort := 12345
	dstPort := 54321
	payload := []byte("hello raw socket world!")

	packet, err := buildIPv4UDPPacket(srcIP, dstIP, srcPort, dstPort, payload)
	if err != nil {
		t.Fatalf("Failed to build IPv4 UDP packet: %v", err)
	}

	// Verify minimum length: 20 (IP) + 8 (UDP) = 28 bytes
	if len(packet) != 28+len(payload) {
		t.Errorf("Expected packet length %d, got %d", 28+len(payload), len(packet))
	}

	// Verify IP Header Checksum
	if cs := checksum(packet[0:20]); cs != 0 {
		t.Errorf("IP header checksum is invalid: got %04x, expected 0", cs)
	}

	// Verify UDP Pseudo-Header Checksum
	// Construct the pseudo header
	udpLen := 8 + len(payload)
	pseudoHeader := make([]byte, 12+udpLen)
	copy(pseudoHeader[0:4], srcIP.To4())
	copy(pseudoHeader[4:8], dstIP.To4())
	pseudoHeader[8] = 0
	pseudoHeader[9] = 17
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(udpLen))
	copy(pseudoHeader[12:], packet[20:])

	if cs := checksum(pseudoHeader); cs != 0 {
		t.Errorf("UDP checksum is invalid: got %04x, expected 0", cs)
	}
}
