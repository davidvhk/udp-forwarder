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