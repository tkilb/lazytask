BINARY  := lazytask
INSTALL_DIR := $(HOME)/bin
CMD_PKG := ./cmd/lazytask

.PHONY: build install test clean

build:
	go build -o $(BINARY) $(CMD_PKG)

## install builds the binary and copies it to $(INSTALL_DIR) (must be on PATH).
install: build
	mkdir -p $(INSTALL_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

test:
	go test ./...

clean:
	rm -f $(BINARY)
