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

func (mt *methodType) newArgv() reflect.Value {
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

func (mt *methodType) newReplyv() reflect.Value {
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
	name     string
	typ      reflect.Type  // 结构体类型
	receiver reflect.Value // 结构体实例
	method   map[string]*methodType
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.receiver, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func newService(rcvr interface{}) *service {
	// 创建服务
	ser := new(service)

	ser.receiver = reflect.ValueOf(rcvr)
	ser.name = reflect.Indirect(ser.receiver).Type().Name() // ??
	ser.typ = reflect.TypeOf(rcvr)
	// 判断是否可以导入
	if !ast.IsExported(ser.name) {
		log.Fatalf("rpc server: %s is not a valid service name", ser.name)
	}
	// 注册方法
	ser.registerMethods()
	return ser

}
