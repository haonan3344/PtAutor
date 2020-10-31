# 开发相关

## 跨平台编译
群晖使用Linux平台对应硬件架构的二进制即可。

### Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ptautor.go
### Win64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ptautor.go
### Win32
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build ptautor.go
