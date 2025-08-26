#!/bin/bash

# 编译当前系统版本
echo "Building for current system..."
go build -ldflags "-s -w" -o ipmi-ssh-tool

# 编译 Windows 版本
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -H=windowsgui" -o ipmi-ssh-tool.exe

# 编译 Linux 版本
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o ipmi-ssh-tool-linux

# 编译 macOS 版本
echo "Building for macOS..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o ipmi-ssh-tool-macos

echo "Build completed:"
ls -lh ipmi-ssh-tool*