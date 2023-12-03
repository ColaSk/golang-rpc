package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

type JsonCodec struct {
	conn io.ReadWriteCloser // socket 链接实例
	buf  *bufio.Writer      // 缓冲区 增加性能
	dec  *json.Decoder      // 解码器
	enc  *json.Encoder      // 编码器
}

/* 实现 Codec 接口*/
func (j *JsonCodec) ReadHeader(h *Header) error {
	return j.dec.Decode(h)
}

func (j *JsonCodec) ReadBody(body interface{}) error {
	return j.dec.Decode(body)
}

func (j *JsonCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = j.buf.Flush()
		if err != nil {
			_ = j.Close()
		}
	}()
	if err := j.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err := j.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (j *JsonCodec) Close() error {
	return j.conn.Close()
}

// ? 确保接口被实现常用的方式
var _ Codec = (*JsonCodec)(nil)

// 返回gob实体指针  gob编码处理机制
func NewJsonCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(buf),
	}
}
