# Settings
APP_NAME = go-socks-proxy
BIN_NAME = socks5-proxy
INSTALL_DIR = /opt/$(APP_NAME)
SYSTEMD_DIR = /etc/systemd/system
SERVICE_NAME = $(APP_NAME).service

.PHONY: all build install systemd clean uninstall status

all: build

# 1. Build the binary without debug information to reduce size
build:
	@echo "==> Compiling project..."
	go build -ldflags="-s -w" -o $(BIN_NAME) main.go
	@echo "==> Build complete: $(BIN_NAME)"

# 2. Create directory and copy the binary
install: build
	@echo "==> Installing to $(INSTALL_DIR)..."
	sudo mkdir -p $(INSTALL_DIR)
	sudo cp $(BIN_NAME) $(INSTALL_DIR)/
	sudo chmod +x $(INSTALL_DIR)/$(BIN_NAME)
	@echo "==> File copied successfully."

# 3. Create and start systemd service safely using line-by-line echo
systemd: install
	@echo "==> Generating systemd config file..."
	@echo "[Unit]" > $(SERVICE_NAME)
	@echo "Description=Go SOCKS5 Proxy" >> $(SERVICE_NAME)
	@echo "After=network.target" >> $(SERVICE_NAME)
	@echo "" >> $(SERVICE_NAME)
	@echo "[Service]" >> $(SERVICE_NAME)
	@echo "Type=simple" >> $(SERVICE_NAME)
	@echo "User=root" >> $(SERVICE_NAME)
	@echo "ExecStart=$(INSTALL_DIR)/$(BIN_NAME)" >> $(SERVICE_NAME)
	@echo "# Uncomment and change variables below if needed" >> $(SERVICE_NAME)
	@echo "Environment=\"SOCKS_PROXY_PORT=1080\"" >> $(SERVICE_NAME)
	@echo "# Environment=\"ALLOWED_IPS=127.0.0.1, 192.168.1.5\"" >> $(SERVICE_NAME)
	@echo "Restart=on-failure" >> $(SERVICE_NAME)
	@echo "RestartSec=5" >> $(SERVICE_NAME)
	@echo "LimitNOFILE=65536" >> $(SERVICE_NAME)
	@echo "" >> $(SERVICE_NAME)
	@echo "[Install]" >> $(SERVICE_NAME)
	@echo "WantedBy=multi-user.target" >> $(SERVICE_NAME)
	@echo "==> Moving configuration to $(SYSTEMD_DIR)..."
	@sudo mv $(SERVICE_NAME) $(SYSTEMD_DIR)/$(SERVICE_NAME)
	@sudo chown root:root $(SYSTEMD_DIR)/$(SERVICE_NAME)
	@echo "==> Reloading systemd daemon and starting service..."
	@sudo systemctl daemon-reload
	@sudo systemctl enable $(SERVICE_NAME)
	@sudo systemctl start $(SERVICE_NAME)
	@echo "==> Done! Service is running."

# Quick status check
status:
	systemctl status $(SERVICE_NAME)

# Clean local compiled files
clean:
	@echo "==> Cleaning up..."
	rm -f $(BIN_NAME)

# Completely remove the program and service from the system
uninstall:
	@echo "==> Removing service and files..."
	-sudo systemctl stop $(SERVICE_NAME)
	-sudo systemctl disable $(SERVICE_NAME)
	sudo rm -f $(SYSTEMD_DIR)/$(SERVICE_NAME)
	sudo systemctl daemon-reload
	sudo rm -rf $(INSTALL_DIR)
	@echo "==> Uninstallation complete."
