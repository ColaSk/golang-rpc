package server

import (
	"encoding/json"
	"errors"
	"gmrpc/codec"
	"gmrpc/service"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

type Option struct {
	CodecType      codec.Type // 解码类型
	MagicNumber    int
	ConnectTimeout time.Duration // int64  default 10 连接超时
	HandleTimeout  time.Duration // int64  default 0  处理超时
}

type request struct {
	h      *codec.Header
	argv   reflect.Value // 反射
	replyv reflect.Value // 反射
	mtype  *service.MethodType
	svc    *service.Service
}

type Server struct {
	serviceMap sync.Map
}

var invalidRequest = struct{}{}

func (server *Server) Register(rcvr interface{}) error {
	s := service.NewService(rcvr)

	if _, loaded := server.serviceMap.LoadOrStore(s.Name, s); loaded {
		return errors.New("rpc: service already defined: " + s.Name)
	}
	return nil
}

func (server *Server) findService(serviceMethod string) (svc *service.Service, mtype *service.MethodType, err error) {
	// 获取分隔符位置
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}

	// 获取服务名称与方法名称
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]

	// 获取服务
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	// 转化服务与方法
	svc = svci.(*service.Service)
	mtype = svc.Method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

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

	// 创建请求
	req := &request{h: header}
	req.svc, req.mtype, err = server.findService(header.ServiceMethod)
	if err != nil {
		return nil, err
	}
	req.argv = req.mtype.NewArgv()
	req.replyv = req.mtype.NewReplyv()

	var argvi any
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	} else {
		argvi = req.argv.Interface()
	}

	// 解析参数
	err = cc.ReadBody(argvi)
	if err != nil {
		log.Println("rpc server read argv err:", err)
		return req, err
	}

	return req, nil

}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	err := req.svc.Call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		server.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
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
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: time.Second * 10,
	HandleTimeout:  0,
}
var DefaultJsonOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.JsonType,
	ConnectTimeout: time.Second * 10,
	HandleTimeout:  0,
}

func Register(rcvr interface{}, server ...*Server) error {
	var ser *Server
	if len(server) >= 1 {
		ser = server[0]
	} else {
		ser = DefaultServer
	}

	return ser.Register(rcvr)
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}
