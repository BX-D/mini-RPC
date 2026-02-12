package transport

import (
	"fmt"
	"net"
	"sync"
)

// transport/pool.go

type ConnPool struct {
	mu       sync.Mutex
	conns    chan *PoolConn // 缓冲 channel 作为池
	addr     string
	maxConns int
	curConns int // 当前已创建的连接数
	factory  func() (net.Conn, error)
}

type PoolConn struct {
	net.Conn
	pool     *ConnPool
	unusable bool // 标记连接是否已损坏
}

// NewConnPool 创建连接池
func NewConnPool(addr string, maxConns int, factory func() (net.Conn, error)) *ConnPool {
	return &ConnPool{
		conns:    make(chan *PoolConn, maxConns),
		addr:     addr,
		maxConns: maxConns,
		factory:  factory,
	}
}

// Get 从池中获取连接
func (p *ConnPool) Get() (*PoolConn, error) {
	select {
	case conn := <-p.conns:
		if conn.unusable {
			return p.createNew()
		}
		return conn, nil
	default:
		// 池空了
		if p.curConns < p.maxConns {
			return p.createNew()
		}
		// 已达上限，阻塞等待
		conn := <-p.conns
		return conn, nil
	}
}

// Put 归还连接到池中
func (p *ConnPool) Put(conn *PoolConn) {
	if conn.unusable {
		conn.Close()
		p.mu.Lock()
		p.curConns--
		p.mu.Unlock()
		return
	}
	p.conns <- conn
}

// Close 关闭连接池，释放所有资源
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	close(p.conns)
	for conn := range p.conns {
		conn.Close()
		p.curConns--
	}
	return nil
}

// createNew 创建新连接
func (p *ConnPool) createNew() (*PoolConn, error) {
	// check if we can create more connectionsc
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.curConns >= p.maxConns {
		return nil, fmt.Errorf("connection pool exhausted")
	}

	netConn, err := p.factory()

	if err != nil {
		return nil, err
	}

	p.curConns++
	return &PoolConn{
		Conn:     netConn,
		pool:     p,
		unusable: false,
	}, nil
}
