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

## Building a Canonical ROCK (Rockcraft / Bare Base)

For ultra-minimal, distroless OCI container deployments with minimal attack surface, **udp-forwarder** supports building as a Canonical **ROCK** image using Rockcraft.

### Why a `bare` Base ROCK?
- **Distroless & Minimal Overhead**: Built on `base: bare` with `build-base: ubuntu@24.04`, containing strictly the compiled Go binary, configuration files, minimal dynamic C runtime libraries (`libc`), and Pebble service manager.
- **Enhanced Security**: No package managers (apt/dpkg), shells, or extraneous binaries that could be leveraged by attackers.
- **Pebble Service Management**: Managed by Canonical's lightweight Pebble init process for process lifecycle and health supervision.

### Prerequisites
Install Rockcraft and LXD on Ubuntu:
```bash
sudo snap install rockcraft --classic
sudo snap install lxd
sudo lxd init --auto
```

### Build the ROCK Container Image
```bash
# Build the .rock artifact in an isolated LXD container
make rock
# or: rockcraft pack
```
This produces the OCI image archive: `udp-forwarder_1.0.0_amd64.rock`.

### Load and Run with Docker / Podman
Import the `.rock` file directly into your local Docker daemon:
```bash
# Using skopeo (available via rockcraft snap or host)
rockcraft.skopeo --insecure-policy copy \
  oci-archive:udp-forwarder_1.0.0_amd64.rock \
  docker-daemon:udp-forwarder:1.0.0

# Run in Standard Forwarding Mode
docker run -d \
  --name udp-forwarder \
  --restart unless-stopped \
  -p 6343:6343/udp \
  -v /path/to/config.yaml:/etc/udp-forwarder/config.yaml:ro \
  udp-forwarder:1.0.0

# Run in Transparent Forwarding Mode (requires host networking or NET_RAW capability)
docker run -d \
  --name udp-forwarder \
  --restart unless-stopped \
  --net=host \
  --cap-add=NET_RAW \
  --cap-add=NET_BIND_SERVICE \
  -v /path/to/config.yaml:/etc/udp-forwarder/config.yaml:ro \
  udp-forwarder:1.0.0
```

### Clean Build Cache
```bash
make rock-clean
# or: rockcraft clean
```

## License

This project is licensed under the MIT License. See the LICENSE file for more details.
