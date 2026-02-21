// Package transport also provides a basic TCP connection pool (ConnPool).
//
// Note: In the current architecture, the Client uses a shared []*ClientTransport pool
// with round-robin selection instead of this borrow/return ConnPool. This ConnPool is
// retained as an alternative approach — it's useful when connections are used exclusively
// (one request at a time per connection) rather than multiplexed.
//
// Pool design: uses a buffered channel as a natural FIFO queue.
// Buffered channels are concurrency-safe, and blocking on empty is built-in.
package transport

import (
	"fmt"
	"net"
	"sync"
)

// ConnPool manages a pool of reusable TCP connections to a single address.
type ConnPool struct {
	mu       sync.Mutex
	conns    chan *PoolConn           // Buffered channel as pool — FIFO, goroutine-safe
	addr     string                   // Target address
	maxConns int                      // Maximum number of connections
	curConns int                      // Currently created connections (may be < maxConns)
	factory  func() (net.Conn, error) // Connection factory function
}

// PoolConn wraps a net.Conn with pool metadata.
type PoolConn struct {
	net.Conn
	pool     *ConnPool
	unusable bool // Marked true when the connection encounters an error
}

// NewConnPool creates a connection pool with the given max size.
// Connections are created lazily — the pool starts empty and grows on demand.
func NewConnPool(addr string, maxConns int, factory func() (net.Conn, error)) *ConnPool {
	return &ConnPool{
		conns:    make(chan *PoolConn, maxConns),
		addr:     addr,
		maxConns: maxConns,
		factory:  factory,
	}
}

// Get retrieves a connection from the pool.
// Strategy:
//  1. Try to get an existing connection from the channel (non-blocking select)
//  2. If pool is empty but under limit, create a new connection
//  3. If pool is empty and at limit, block until one is returned
func (p *ConnPool) Get() (*PoolConn, error) {
	select {
	case conn := <-p.conns:
		if conn.unusable {
			return p.createNew()
		}
		return conn, nil
	default:
		// Pool is empty
		if p.curConns < p.maxConns {
			return p.createNew()
		}
		// At capacity — block until a connection is returned
		conn := <-p.conns
		return conn, nil
	}
}

// Put returns a connection to the pool.
// If the connection is marked unusable (error occurred), it's closed and discarded.
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

// Close shuts down the pool and closes all connections.
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

// createNew creates a new TCP connection via the factory function.
// Protected by mutex to prevent exceeding maxConns under concurrent access.
func (p *ConnPool) createNew() (*PoolConn, error) {
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
