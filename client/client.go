package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gmrpc/codec"
	"gmrpc/server"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// rpc调用结构体
type Call struct {
	Seq           uint64
	ServiceMethod string      // 服务方法名
	Args          interface{} // 参数
	Reply         interface{} // 结果
	Error         error       // 错误信息
	Done          chan *Call  // 支持异步调用  chan 通道 用于协程通信
}

func (call *Call) done() {
	// 调用结束被执行
	call.Done <- call
}

type Client struct {
	cc       codec.Codec
	opt      *server.Option
	header   codec.Header
	sending  sync.Mutex // 互斥锁，保证请求有序
	mu       sync.Mutex
	seq      uint64           // 请求编号
	pending  map[uint64]*Call // 存储未处理完成的call实例
	closing  bool             // 用户主动关闭标志
	shutdown bool             // 错误发生标志
}

var _ io.Closer = (*Client)(nil)
var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	defer client.mu.Unlock()
	client.mu.Lock()

	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	// 注册调用
	defer client.mu.Unlock()
	client.mu.Lock()

	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}

	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil

}

func (client *Client) removeCall(seq uint64) *Call {
	// 删除调用
	defer client.mu.Unlock()
	client.mu.Lock()

	var call *Call = client.pending[seq]
	delete(client.pending, seq)
	return call

}

func (client *Client) terminateCalls(err error) {
	// 客户端或服务端发生错误时，将错误信息传递给call
	defer client.sending.Unlock()
	defer client.mu.Unlock()
	client.sending.Lock()
	client.mu.Lock()

	client.shutdown = true

	for _, call := range client.pending {
		call.Error = err
		call.done()
	}

}

func (client *Client) receive() {
	// 接收数据

	var err error

	for err == nil {

		// 读取请求头
		var header codec.Header
		if err = client.cc.ReadBody(&header); err != nil {
			break
		}

		var call *Call = client.removeCall(header.Seq)

		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)
		case header.Error != "":
			call.Error = errors.New(header.Error)
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err)
}

func (client *Client) send(call *Call) {
	// 发送数据
	defer client.sending.Unlock()
	client.sending.Lock()

	// 注册
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 更新请求头
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// 发送数据
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	// 异步调用
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 同步调用
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))

	// 上下文控制超时
	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}

func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	// 创建客户端
	/*
		1. 与服务端协商协议交换
		2. 接收响应
	*/
	// 定义协议
	_func := codec.NewCodecFuncMap[opt.CodecType]

	if _func == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}

	// 协商协议
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}

	return newClientCodec(_func(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *server.Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *server.Option) (client *Client, err error)

func dialTimeout(f newClientFunc, network string, address string, opts ...*server.Option) (client *Client, err error) {
	// 超时处理

	// 创建opt
	var opt *server.Option = server.DefaultOption
	if len(opts) >= 1 && opts[0] != nil {
		opt = opts[0]
	}

	// 创建链接 连接超时处理
	// conn, err := net.Dial(network, address)
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)

	if err != nil {
		return nil, err
	}

	// 关闭连接
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	clientResCh := make(chan clientResult)

	go func() {
		client, err := f(conn, opt)
		clientresult := clientResult{client: client, err: err}
		clientResCh <- clientresult
	}()

	if opt.ConnectTimeout == 0 {
		clientRes := <-clientResCh
		return clientRes.client, clientRes.err
	}

	select {
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
	case result := <-clientResCh:
		return result.client, result.err
	}
}

func Dial(network string, address string, opts ...*server.Option) (*Client, error) {
	return dialTimeout(NewClient, network, address, opts...)
}
