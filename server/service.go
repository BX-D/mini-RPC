package server

import (
	"fmt"
	"reflect"
)

// methodType stores the reflection metadata for a single RPC-compatible method.
type methodType struct {
	method    reflect.Method // The reflected method itself
	ArgType   reflect.Type   // Type of the first argument (e.g., *Args → Args)
	ReplyType reflect.Type   // Type of the second argument (e.g., *Reply → Reply)
}

// service wraps a user-defined struct (e.g., &Arith{}) and its RPC-compatible methods.
// It maps method names to their reflection metadata for dynamic dispatch.
type service struct {
	name   string                 // Service name, derived from struct name (e.g., "Arith")
	rcvr   reflect.Value          // The receiver value (pointer to struct instance)
	typ    reflect.Type           // The receiver type (pointer type)
	method map[string]*methodType // Method name → reflection metadata
}

// NewService creates a service from a pointer to a struct.
// It validates the receiver and scans all methods for RPC-compatible signatures.
//
// Example:
//
//	svc, err := NewService(&Arith{})
//	// svc.name == "Arith"
//	// svc.method["Add"] == &methodType{...}
func NewService(rcvr any) (*service, error) {
	typ := reflect.TypeOf(rcvr)

	// Must be a pointer (methods with pointer receivers won't show up on value types)
	if typ.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("rpc: rcvr must be a pointer, got %s", typ.Kind())
	}
	// Must point to a struct
	if typ.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("rpc: rcvr must point to a struct, got %s", typ.Elem().Kind())
	}

	val := reflect.ValueOf(rcvr)
	srv := &service{
		name:   typ.Elem().Name(), // Use struct name as service name
		rcvr:   val,
		typ:    typ,
		method: make(map[string]*methodType),
	}
	srv.RegisterMethods()
	return srv, nil
}

// errorType is used to check if a method's return type is `error`.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// RegisterMethods scans all exported methods of the struct and registers those
// that match the RPC method signature convention:
//
//	func (receiver) MethodName(args *ArgsType, reply *ReplyType) error
//
// Requirements:
//   - Exactly 3 input params: receiver, *Args, *Reply (both must be pointers)
//   - Exactly 1 output: error
//
// Methods that don't match are silently skipped.
func (s *service) RegisterMethods() {
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)

		// Filter: must have 3 inputs (receiver + args + reply), 1 output (error)
		if method.Type.NumIn() != 3 || method.Type.NumOut() != 1 {
			continue
		}
		// Output must be error type
		if method.Type.Out(0) != errorType {
			continue
		}
		// Both args and reply must be pointer types
		if method.Type.In(1).Kind() != reflect.Ptr || method.Type.In(2).Kind() != reflect.Ptr {
			continue
		}

		// Register the method — store Elem() types (not pointer types)
		// so we can later use reflect.New() to create instances
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   method.Type.In(1).Elem(), // *Args → Args
			ReplyType: method.Type.In(2).Elem(), // *Reply → Reply
		}
	}
}

// Call invokes the registered method via reflection.
//
//	svc.Call(method, reflect.New(ArgsType), reflect.New(ReplyType))
//
// The reflect.Value args must be pointer values (created via reflect.New).
func (s *service) Call(mType *methodType, argv, replyv reflect.Value) error {
	args := [3]reflect.Value{s.rcvr, argv, replyv}
	results := mType.method.Func.Call(args[:])

	// Check if the returned error is non-nil
	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}
