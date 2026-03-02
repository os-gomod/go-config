// Package pool provides optimized sync.Pool implementations for the configuration system.
package pool

import (
	"strings"
	"sync"
)

// BytePool provides a pool of byte slices.
type BytePool struct {
	pool sync.Pool
	size int
}

// NewBytePool creates a new byte slice pool.
func NewBytePool(size int) *BytePool {
	return &BytePool{
		size: size,
		pool: sync.Pool{
			New: func() any {
				b := make([]byte, 0, size)

				return &b
			},
		},
	}
}

// Get retrieves a byte slice from the pool.
func (p *BytePool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

// Put returns a byte slice to the pool.
func (p *BytePool) Put(b *[]byte) {
	*b = (*b)[:0]
	p.pool.Put(b)
}

// StringPool provides a pool of strings.Builder.
type StringPool struct {
	pool sync.Pool
}

// NewStringPool creates a new string builder pool.
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() any {
				return new(strings.Builder)
			},
		},
	}
}

// Get retrieves a strings.Builder from the pool.
func (p *StringPool) Get() *strings.Builder {
	sb := p.pool.Get().(*strings.Builder)
	sb.Reset()

	return sb
}

// Put returns a strings.Builder to the pool.
func (p *StringPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

// MapPool provides a pool of maps.
type MapPool[K comparable, V any] struct {
	pool sync.Pool
	size int
}

// NewMapPool creates a new map pool.
func NewMapPool[K comparable, V any](size int) *MapPool[K, V] {
	return &MapPool[K, V]{
		size: size,
		pool: sync.Pool{
			New: func() any {
				return make(map[K]V, size)
			},
		},
	}
}

// Get retrieves a map from the pool.
func (p *MapPool[K, V]) Get() map[K]V {
	return p.pool.Get().(map[K]V)
}

// Put returns a map to the pool.
func (p *MapPool[K, V]) Put(m map[K]V) {
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	p.pool.Put(m)
}

// SlicePool provides a pool of slices.
type SlicePool[T any] struct {
	pool sync.Pool
	size int
}

// NewSlicePool creates a new slice pool.
func NewSlicePool[T any](size int) *SlicePool[T] {
	return &SlicePool[T]{
		size: size,
		pool: sync.Pool{
			New: func() any {
				slice := make([]T, 0, size)

				return &slice
			},
		},
	}
}

// Get retrieves a slice from the pool.
func (p *SlicePool[T]) Get() *[]T {
	return p.pool.Get().(*[]T)
}

// Put returns a slice to the pool.
func (p *SlicePool[T]) Put(s *[]T) {
	*s = (*s)[:0]
	p.pool.Put(s)
}

// Buffer is a growable buffer with pool support.
type Buffer struct {
	data []byte
	pool *BytePool
}

// NewBuffer creates a new buffer.
func NewBuffer(pool *BytePool) *Buffer {
	return &Buffer{
		data: *pool.Get(),
		pool: pool,
	}
}

// Write appends bytes to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)

	return len(p), nil
}

// WriteByte appends a byte to the buffer.
func (b *Buffer) WriteByte(c byte) error {
	b.data = append(b.data, c)

	return nil
}

// WriteString appends a string to the buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	b.data = append(b.data, s...)

	return len(s), nil
}

// Bytes returns the buffer contents.
func (b *Buffer) Bytes() []byte {
	return b.data
}

// String returns the buffer contents as a string.
func (b *Buffer) String() string {
	return string(b.data)
}

// Reset clears the buffer.
func (b *Buffer) Reset() {
	b.data = b.data[:0]
}

// Release returns the buffer to the pool.
func (b *Buffer) Release() {
	if b.pool != nil {
		b.pool.Put(&b.data)
		b.data = nil
	}
}

// Len returns the buffer length.
func (b *Buffer) Len() int {
	return len(b.data)
}

// Cap returns the buffer capacity.
func (b *Buffer) Cap() int {
	return cap(b.data)
}
