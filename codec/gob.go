package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

/*
管理 gobs 流 - 在编码器（发送器）和解码器（接收器）之间交换的二进制值。
一个典型的用途是传输远程过程调用（RPC）的参数和结果
*/

// 定义gob 类型
type GobCodec struct {
	conn io.ReadWriteCloser // socket 链接实例
	buf  *bufio.Writer      // 缓冲区 增加性能
	dec  *gob.Decoder       // 解码器
	enc  *gob.Encoder       // 编码器
}

/* 实现 Codec 接口*/
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}

// ? 确保接口被实现常用的方式
var _ Codec = (*GobCodec)(nil)

// 返回gob实体指针  gob编码处理机制
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}
