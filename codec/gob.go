package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// 定义gob 类型
type GobCodec struct {
	conn io.ReadWriteCloser // socket 链接实例
	buf  *bufio.Writer      // 缓冲区
	dec  *gob.Decoder       //
	enc  *gob.Encoder
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

// 返回gob实体指针
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}
