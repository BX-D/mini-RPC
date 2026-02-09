package server

import (
	"fmt"
	"reflect"
)

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

type service struct {
	name   string
	rcvr   reflect.Value
	typ    reflect.Type
	method map[string]*methodType
}

// NewService 创建 service 并扫描所有合法方法
func NewService(rcvr any) (*service, error) {
	// 1. 用 reflect.TypeOf / ValueOf 获取类型和值
	typ := reflect.TypeOf(rcvr)
	if typ.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("rpc: rcvr must be a pointer, got %s", typ.Kind())
	}
	if typ.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("rpc: rcvr must point to a struct, got %s", typ.Elem().Kind())
	}
	val := reflect.ValueOf(rcvr)
	// 2. 用类型名作为 service name
	srv := &service{
		name:   typ.Elem().Name(),
		rcvr:   val,
		typ:    typ,
		method: make(map[string]*methodType),
	}
	// 3. 调用 registerMethods() 扫描方法
	srv.RegisterMethods()

	return srv, nil
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// registerMethods 扫描 struct 的导出方法，过滤出符合 RPC 签名的
func (s *service) RegisterMethods() {
	// 遍历所有方法，合法条件：
	//   - 3 个入参: (receiver, *Args, *Reply)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		if method.Type.NumIn() != 3 || method.Type.NumOut() != 1 || method.Type.Out(0) != errorType ||
			method.Type.In(1).Kind() != reflect.Ptr || method.Type.In(2).Kind() != reflect.Ptr {
			continue
		}

		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   method.Type.In(1).Elem(),
			ReplyType: method.Type.In(2).Elem(),
		}
	}
}

// call 通过反射调用方法
func (s *service) Call(mType *methodType, argv, replyv reflect.Value) error {
	args := [3]reflect.Value{s.rcvr, argv, replyv}
	results := mType.method.Func.Call(args[:])
	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}
