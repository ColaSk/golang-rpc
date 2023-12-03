package codec

import "io"

// 定义头部
type Header struct {
	ServiceMethod string // 调用包方法名称 Service.Method
	Seq           uint64 // 请求序列号
	Error         string // 错误信息
}

// 对消息体编解码接口
type Codec interface {
	io.Closer // 继承关闭资源的接口
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// Codec构造方法
// 定义类型
type NewCodecFunc func(io.ReadWriteCloser) Codec
type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json" // not implemented
)

var NewCodecFuncMap map[Type]NewCodecFunc

// 包的初始化
func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
	NewCodecFuncMap[JsonType] = NewJsonCodec
}
