package server

import (
	"encoding/json"
	"fmt"
	"gmrpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

type Option struct {
	CodecType   codec.Type // 解码类型
	MagicNumber int
}

type request struct {
	h      *codec.Header
	argv   reflect.Value // 反射
	replyv reflect.Value // 反射
}

type Server struct{}

var invalidRequest = struct{}{}

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()

		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}

		go server.ServeConn(conn)
	}
}

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { conn.Close() }() // 析构

	var opt Option

	err := json.NewDecoder(conn).Decode(&opt)
	if err != nil {
		log.Println("rpc server [opt] err: ", err)
		return
	}

	_func := codec.NewCodecFuncMap[opt.CodecType]
	if _func == nil {
		log.Println("rpc server [codec type] err: ", opt.CodecType)
		return
	}

	server.ServeCodec(_func(conn))
}

func (server *Server) ServeCodec(cc codec.Codec) {
	// 1. 读取请求
	// 2. 处理请求
	// 3. 回复请求

	sending := new(sync.Mutex) // 互斥锁
	wg := new(sync.WaitGroup)  // 等待一组 goroutine 结束

	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	cc.Close()

}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var header codec.Header
	err := cc.ReadHeader(&header)
	if err != nil {
		log.Println("rpc server read header error:", err)
		return nil, err
	}

	return &header, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {

	// 解析请求头
	header, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{h: header}
	req.argv = reflect.New(reflect.TypeOf(""))

	// 解析参数
	err = cc.ReadBody(req.argv.Interface())
	if err != nil {
		log.Println("rpc server read argv err:", err)
	}

	return req, nil

}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	defer sending.Unlock()
	sending.Lock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

// 服务端构造函数
func NewServer() *Server {
	return &Server{}
}

var DefaultServer *Server = NewServer()
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}
var DefaultJsonOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.JsonType,
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}
