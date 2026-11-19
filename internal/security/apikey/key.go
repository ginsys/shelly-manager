package apikey

import "sync/atomic"

// cell holds one key value. Handles that share a cell see every change at the same instant.
type cell struct{ v atomic.Pointer[string] }

// Key is a handle to a replaceable key. The zero value is an unset key and is ready to use. Every
// operation is atomic, so a key can be read by requests while another request replaces it.
//
// Share makes two handles point at one cell, so a single Set switches every guard that holds one of
// them. A Key must not be copied after first use (go vet checks this).
type Key struct{ c atomic.Pointer[cell] }

// ensure returns the key's cell and allocates it on first use. Two goroutines that race here end up
// with the same cell.
func (k *Key) ensure() *cell {
	if c := k.c.Load(); c != nil {
		return c
	}
	k.c.CompareAndSwap(nil, new(cell))
	return k.c.Load()
}

// Get returns the current key, or "" while none is set.
func (k *Key) Get() string {
	c := k.c.Load()
	if c == nil {
		return ""
	}
	if v := c.v.Load(); v != nil {
		return *v
	}
	return ""
}

// Set replaces the key for every handle that shares this cell.
func (k *Key) Set(key string) { k.ensure().v.Store(&key) }

// Share makes k use src's cell from now on. Call it when the handlers are assembled: k's own earlier
// value is dropped, src's stays.
func (k *Key) Share(src *Key) { k.c.Store(src.ensure()) }
