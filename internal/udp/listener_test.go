package udp

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/davidvhk/udp-forwarder/internal/forwarder"
)

func getFreeUDPPort(t *testing.T) int {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve addr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func TestListener_StartListening_NoRace(t *testing.T) {
	// 1. Get a free port for the listener
	listenerPort := getFreeUDPPort(t)
	listenerAddrStr := fmt.Sprintf("127.0.0.1:%d", listenerPort)

	// 2. Set up a destination UDP listener
	destConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("Failed to listen on destination port: %v", err)
	}
	defer destConn.Close()
	destAddrStr := destConn.LocalAddr().String()

	// 3. Set up Forwarder and Listener
	fwd := &forwarder.Forwarder{}
	fwd.AddDestination(destAddrStr)

	l := &Listener{}

	// Start listening in a background goroutine
	go func() {
		_ = l.StartListening(listenerAddrStr, fwd)
	}()

	// Allow listener goroutine to start
	time.Sleep(100 * time.Millisecond)

	// 4. Send packets concurrently
	numPackets := 100
	var wg sync.WaitGroup

	// Set up receiving map to count received payloads and detect corruption
	receivedPayloads := make(map[string]int)
	var mapMu sync.Mutex

	// Start receiver goroutine
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		buf := make([]byte, 1024)
		for i := 0; i < numPackets; i++ {
			_ = destConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := destConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			payload := string(buf[:n])
			mapMu.Lock()
			receivedPayloads[payload]++
			mapMu.Unlock()
		}
	}()

	// Send packets concurrently from separate goroutines
	listenerUDPAddr, err := net.ResolveUDPAddr("udp", listenerAddrStr)
	if err != nil {
		t.Fatalf("Failed to resolve listener addr: %v", err)
	}

	for i := 0; i < numPackets; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			senderConn, err := net.DialUDP("udp", nil, listenerUDPAddr)
			if err != nil {
				return
			}
			defer senderConn.Close()

			payload := fmt.Sprintf("message-payload-number-%d", id)
			_, _ = senderConn.Write([]byte(payload))
		}(i)
	}

	wg.Wait()

	// Wait for receiver to finish
	<-doneChan

	// Validate received payloads
	mapMu.Lock()
	defer mapMu.Unlock()

	if len(receivedPayloads) != numPackets {
		t.Errorf("Expected to receive %d unique payloads, but got %d", numPackets, len(receivedPayloads))
	}

	for i := 0; i < numPackets; i++ {
		expectedPayload := fmt.Sprintf("message-payload-number-%d", i)
		count, ok := receivedPayloads[expectedPayload]
		if !ok {
			t.Errorf("Missing expected payload: %q", expectedPayload)
		} else if count != 1 {
			t.Errorf("Payload %q received %d times, expected 1", expectedPayload, count)
		}
	}
}
