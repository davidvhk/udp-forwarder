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

The application requires a configuration file (`config/config.yaml`) to specify the listening address and the list of destination addresses for forwarding UDP traffic. 

### Example Configuration

```yaml
listen_address: "0.0.0.0:8080"
destinations:
  - "192.168.1.100:8081"
  - "192.168.1.101:8082"
```

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