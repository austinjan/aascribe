APP_NAME := aascribe
CMD_PATH := ./cmd/aascribe
DIST_DIR := dist
GO ?= go

CURRENT_BINARY := $(DIST_DIR)/$(APP_NAME)

.PHONY: all release clean

all:
	mkdir -p $(DIST_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(CURRENT_BINARY) $(CMD_PATH)

release:
	mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)_windows_amd64.exe $(CMD_PATH)
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)_linux_amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)_macos_arm64 $(CMD_PATH)

clean:
	rm -rf $(DIST_DIR)
