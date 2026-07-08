# UDP Forwarder

This project is a UDP traffic forwarder that listens for incoming UDP packets and forwards them to specified destinations based on a configuration file.

## Project Structure

```
udp-forwarder
├── cmd
│   └── udp-forwarder
│       └── main.go          # Entry point of the application
├── config
│   └── config.yaml          # Configuration file for listening and forwarding addresses
├── internal
│   ├── forwarder
│   │   └── forwarder.go     # Logic for forwarding UDP packets
│   └── udp
│       └── listener.go      # UDP listener implementation
├── go.mod                    # Module definition and dependencies
└── README.md                 # Project documentation
```

## Configuration

The application requires a configuration file (`config/config.yaml`) to specify the listening address, the list of destination addresses for forwarding UDP traffic, and whether transparent mode should be used. 

### Example Configuration

```yaml
listen_address: "0.0.0.0:8080"
transparent: false
mtu: 1500
destinations:
  - "192.168.1.100:8081"
  - "192.168.1.101:8082"
```

### Transparent Mode

By default (`transparent: false`), the forwarder rewrites the source IP of the forwarded packets to the forwarder's own IP address.

When enabling `transparent: true`:
* The forwarder preserves the originating sender's source IP and source port when sending packets to the destinations.
* This is useful for receivers (such as sFlow or syslog collectors) that rely on the originating device's IP address to identify the source of the traffic.
* **Requirements:** Using transparent mode requires the application to run with elevated privileges (as `root` or with the `CAP_NET_RAW` capability on Linux) to create raw sockets.
* **Fallback:** If raw socket creation fails (e.g. if run by a non-root user), the application will print a warning log and fall back to standard forwarding.

### MTU (Maximum Transmission Unit) Configuration

When using `transparent: true`, the forwarder creates raw IP sockets. On Linux/Unix, the kernel does not perform automatic IP fragmentation for raw sockets with custom IP headers (`IP_HDRINCL`). Consequently, any raw packet larger than the MTU of the outgoing network interface will fail to send and return a `message too long` (`EMSGSIZE`) error.

To resolve this:
* Set the `mtu` field in the configuration file to match the MTU of the egress network interface on the machine running the forwarder (e.g., standard Ethernet is `1500`, but virtual bridges or virtual networks/VPNs might use a lower MTU like `1400` or `1376`).
* When the packet size (UDP payload + headers) exceeds the configured MTU, the forwarder will manually perform IP fragmentation in user-space. This splits the packet into valid IP fragments that fit the interface limits, allowing transparent forwarding to work smoothly.
* If `mtu` is omitted or set to `<= 0`, it defaults to `1500`.

## Running the Application

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/udp-forwarder.git
   cd udp-forwarder
   ```

2. Install dependencies:
   ```
   go mod tidy
   ```

3. Create a configuration file (`config/config.yaml`) with the desired settings.

4. Run the application:
   ```
   go run cmd/udp-forwarder/main.go
   ```

## License

This project is licensed under the MIT License. See the LICENSE file for more details.
