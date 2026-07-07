package forwarder

import (
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
