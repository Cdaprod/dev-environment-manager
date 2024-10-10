.PHONY: build install clean

# Variables
BINARY_NAME=ops
SOURCE=dev-environment-manager.go
INSTALL_DIR=/usr/local/bin

# Build the executable
build:
	go build -o $(BINARY_NAME) main.go cmd.go pkg.go

# Install the executable by moving it to the install directory
install: build
	sudo mv $(BINARY_NAME) $(INSTALL_DIR)/

# Clean up build artifacts
clean:
	rm -f $(BINARY_NAME)