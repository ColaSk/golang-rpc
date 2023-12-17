# golang-rpc

## 服务端

### 服务端

- 服务

### 消息编码

- 使用 encoding/gob 序列化反序列化  https://pkg.go.dev/encoding/gob
- 使用 encoding/json 序列化反序列化 https://pkg.go.dev/encoding/json

## 功能

### 服务注册

### 超时处理

- 客户端处理超时
  * 连接超时
  * 发送超时
  * 等待处理超时
  * 接收超时
- 服务端处理超时
  * 读请求超时
  * 发送超时
  * 处理超时
