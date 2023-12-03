package gmrpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method // 方法本身
	ArgType   reflect.Type   // 第一个参数类型
	ReplyType reflect.Type   // 第二个参数类型
	numCalls  uint64         // 统计次数
}

func (mt *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&mt.numCalls)
}

// 创建实例
func (mt *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	// arg可以是指针类型也可以是值类型
	// Kind 获取参数种类
	// Elem 方法获取这个指针指向的元素类型
	if mt.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mt.ArgType.Elem())
	} else {
		argv = reflect.New(mt.ArgType).Elem()
	}
	return argv
}

//创建实例
func (mt *methodType) newReplyv() reflect.Value {
	// reply 必须是指针类型
	replyv := reflect.New(mt.ReplyType.Elem())

	// 获取指针类型的指向的的类型种类
	switch mt.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mt.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mt.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

// 服务类
type service struct {
	name   string                 // 服务名称
	typ    reflect.Type           // 结构体类型
	rcvr   reflect.Value          // 结构体实例本身
	method map[string]*methodType // 储映射的结构体的所有符合条件的方法
}

// 服务调用
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func newService(rcvr interface{}) *service {
	s := new(service)
	// 初始化 服务参数
	s.rcvr = reflect.ValueOf(rcvr)                  // 实例
	s.name = reflect.Indirect(s.rcvr).Type().Name() // 服务名称
	s.typ = reflect.TypeOf(rcvr)                    // 结构体类型
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	// 过滤出符合条件的注册方法
	s.registerMethods()
	return s
}

//  过滤出了符合条件的方法
func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	// 遍历方法
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type

		// 过滤
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		// 获取参数类型与结果类型
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}

		// 注册方法
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
