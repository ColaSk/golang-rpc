package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

/*服务注册
 */

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64
}

func (mt *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&mt.numCalls)
}

func (mt *methodType) NewArgv() reflect.Value {
	// 返回参数实例
	/*
		Elem返回该类型元素类型
	*/
	if mt.ArgType.Kind() == reflect.Ptr {
		// 指针类型
		return reflect.New(mt.ArgType.Elem())
	}
	return reflect.New(mt.ArgType).Elem()
}

func (mt *methodType) NewReplyv() reflect.Value {
	// 返回结果实例
	replyv := reflect.New(mt.ReplyType.Elem())
	switch mt.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mt.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mt.ReplyType.Elem(), 0, 0))
	}

	return replyv
}

type service struct {
	Name     string
	typ      reflect.Type  // 结构体类型
	receiver reflect.Value // 结构体实例
	Method   map[string]*methodType
}

func (s *service) registerMethods() {
	// 注册方法
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type

		// 输入输出判断数量
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		//输出为error判断
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		// 判断导出类型与构建类型
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.Method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.Name, method.Name)
	}
}

func (s *service) Call(m *methodType, argv, replyv reflect.Value) error {
	// 服务调用
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.receiver, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	// 判断导出类型与构建类型
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func NewService(rcvr interface{}) *service {
	// 创建服务
	ser := &service{
		Name:     reflect.Indirect(reflect.ValueOf(rcvr)).Type().Name(), // Indirect 为了兼容指针类型
		typ:      reflect.TypeOf(rcvr),
		receiver: reflect.ValueOf(rcvr),
		Method:   make(map[string]*methodType),
	}

	// 判断是否可以导入
	if !ast.IsExported(ser.Name) {
		log.Fatalf("rpc server: %s is not a valid service name", ser.Name)
	}
	// 注册方法
	ser.registerMethods()
	return ser
}

type MethodType = methodType
type Service = service
