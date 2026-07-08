.PHONY: all build static run clean test install install-service uninstall rpm rpm-static deb help

BINARY_NAME=udp-forwarder
BUILD_DIR=bin
INSTALL_DIR=/usr/local/bin
SYSTEMD_DIR=/etc/systemd/system
CONFIG_DIR=/etc/udp-forwarder
RPMBUILD_DIR=$(PWD)/rpmbuild
DEB_DIR=$(PWD)/debbuild
DEB_ARCH=$(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/' -e 's/armv7l/armhf/')

all: build

help:
	@echo "Available Makefile targets:"
	@echo "  make build             - Compile the Go binary to bin/udp-forwarder"
	@echo "  make static            - Compile a statically linked Go binary to bin/udp-forwarder"
	@echo "  make run               - Build and run the application locally"
	@echo "  make test              - Run tests"
	@echo "  make clean             - Clean up build directories, RPMs, and DEBs"
	@echo "  make rpm               - Build an RPM package (requires rpmbuild)"
	@echo "  make rpm-static        - Build an RPM package with a statically linked binary"
	@echo "  make deb               - Build a DEB package (requires dpkg-deb)"
	@echo "  make install           - Install binary and configuration files to system (requires sudo/root)"
	@echo "  make install-service   - Install and reload systemd service (requires sudo/root)"
	@echo "  make uninstall         - Remove binary, configuration, and systemd service (requires sudo/root)"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/udp-forwarder/main.go

static:
	@echo "Building statically linked $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags "-extldflags -static" -o $(BUILD_DIR)/$(BINARY_NAME) cmd/udp-forwarder/main.go

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR) $(RPMBUILD_DIR) $(DEB_DIR) *.rpm *.deb

test:
	go test ./...

rpm: build
	@echo "Building RPM package..."
	@mkdir -p $(RPMBUILD_DIR)/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	rpmbuild -bb \
		--define "_topdir $(RPMBUILD_DIR)" \
		--define "_sourcedir $(PWD)" \
		udp-forwarder.spec
	@mv $(RPMBUILD_DIR)/RPMS/*/*.rpm .
	@rm -rf $(RPMBUILD_DIR)
	@echo "RPM package built successfully."

rpm-static: static
	@echo "Building RPM package with statically linked binary..."
	@mkdir -p $(RPMBUILD_DIR)/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	rpmbuild -bb \
		--define "_topdir $(RPMBUILD_DIR)" \
		--define "_sourcedir $(PWD)" \
		udp-forwarder.spec
	@mv $(RPMBUILD_DIR)/RPMS/*/*.rpm .
	@rm -rf $(RPMBUILD_DIR)
	@echo "Statically linked RPM package built successfully."

deb: build
	@echo "Building DEB package..."
	@mkdir -p $(DEB_DIR)/DEBIAN
	@mkdir -p $(DEB_DIR)/usr/local/bin
	@mkdir -p $(DEB_DIR)/etc/udp-forwarder
	@mkdir -p $(DEB_DIR)/lib/systemd/system
	@install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DEB_DIR)/usr/local/bin/$(BINARY_NAME)
	@install -m 644 config/config.yaml $(DEB_DIR)/etc/udp-forwarder/config.yaml
	@install -m 644 $(BINARY_NAME).service $(DEB_DIR)/lib/systemd/system/$(BINARY_NAME).service
	@echo "Package: $(BINARY_NAME)" > $(DEB_DIR)/DEBIAN/control
	@echo "Version: 1.0.3" >> $(DEB_DIR)/DEBIAN/control
	@echo "Section: utils" >> $(DEB_DIR)/DEBIAN/control
	@echo "Priority: optional" >> $(DEB_DIR)/DEBIAN/control
	@echo "Architecture: $(DEB_ARCH)" >> $(DEB_DIR)/DEBIAN/control
	@echo "Maintainer: David <david@example.com>" >> $(DEB_DIR)/DEBIAN/control
	@echo "Description: A UDP traffic forwarder" >> $(DEB_DIR)/DEBIAN/control
	@echo " This project is a UDP traffic forwarder that listens for incoming UDP packets and forwards them to specified destinations based on a configuration file." >> $(DEB_DIR)/DEBIAN/control
	@echo "/etc/udp-forwarder/config.yaml" > $(DEB_DIR)/DEBIAN/conffiles
	dpkg-deb --build $(DEB_DIR) $(BINARY_NAME)_1.0.3_$(DEB_ARCH).deb
	@rm -rf $(DEB_DIR)
	@echo "DEB package built successfully."

install: build
	@echo "Installing binary to $(INSTALL_DIR)..."
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installing configuration directory to $(CONFIG_DIR)..."
	mkdir -p $(CONFIG_DIR)
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		cp config/config.yaml $(CONFIG_DIR)/config.yaml; \
		echo "Copied default config.yaml to $(CONFIG_DIR)/config.yaml"; \
	else \
		echo "Configuration already exists at $(CONFIG_DIR)/config.yaml, skipping copy"; \
	fi

install-service:
	@echo "Installing systemd service..."
	install -m 644 $(BINARY_NAME).service $(SYSTEMD_DIR)/$(BINARY_NAME).service
	systemctl daemon-reload
	@echo "Service installed. Use 'systemctl enable --now $(BINARY_NAME)' to start it."

uninstall:
	@echo "Uninstalling binary..."
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalling service..."
	rm -f $(SYSTEMD_DIR)/$(BINARY_NAME).service
	systemctl daemon-reload
	@echo "Removing configuration directory..."
	rm -rf $(CONFIG_DIR)
	@echo "Uninstall complete."
