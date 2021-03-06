package main

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/bblfsh/sdk/v3/uast"
	"github.com/bblfsh/sdk/v3/uast/nodes"
	"github.com/bblfsh/sdk/v3/uast/query"
	"github.com/bblfsh/sdk/v3/uast/query/xpath"
)

// Handle is an opaque int handle used as a reference to other objects either on the Go side or on the client side.
//
// This is required for most Go-based objects since Go 1.6+ has a strict rule that C cannot store any Go pointers.
type Handle uintptr

// IsValue checks if a node is one of the value types.
func IsValue(n Node) bool {
	if n == nil {
		return false
	}
	k := n.Kind()
	return k.In(nodes.KindsValues)
}

// Base is an interface for a basic object with a handle.
type Base interface {
	Handle() Handle
}

// Node is an extension of the External node interface that can also return a handle.
type Node interface {
	Base
	nodes.External
}

// Object is an extension of the ExternalObject interface that can also return a handle.
type Object interface {
	Base
	nodes.ExternalObject
}

// Array is an extension of the ExternalArray interface that can also return a handle.
type Array interface {
	Base
	nodes.ExternalArray
}

// NodeInface is an interface for a node implementation in either Go or the client.
type NodeIface interface {
	// Free releases all the resources associated with this implementation.
	Free()
	// AsNode casts a handle to the node. A handle should be acquired from the same implementation.
	AsNode(h Handle) Node
	// AsTmpNode casts a handle to the temporary node. A handle should be acquired from the same implementation.
	AsTmpNode(h Handle) TmpNode

	// NewObject creates a handle to the new TmpNode that represents a object of a given size.
	// Handle should be passed to AsTmpNode, populated with SetKeyValue and converted to a proper node using Build.
	NewObject(sz int) Handle

	// NewArray creates a handle to the new TmpNode that represents an array of a given size.
	// Handle should be passed to AsTmpNode, populated with SetValue and converted to a proper node using Build.
	NewArray(sz int) Handle

	// NewValue creates a node with a given value. Passing nil always produces a nil node.
	NewValue(v nodes.Value) Node
}

// TmpNode is an interface to a temporary node that is being created. Node should be populated with either
// SetValue (for Arrays) or SetKeyValue (for Objects) and created using Build.
type TmpNode interface {
	// SetValue sets a value on a given index of an Array.
	SetValue(i int, v Node)
	// SetKeyValue sets a value on a given key of an Object.
	SetKeyValue(k string, v Node)
	// Build convert the temporary node into a valid node implementation. The node cannot be changed after this operation,
	// and is guaranteed to preserve its handle.
	Build() Node
}

var ctxes = struct {
	sync.RWMutex
	last     Handle
	byHandle map[Handle]*Context
}{
	byHandle: make(map[Handle]*Context),
}

func newContext(impl NodeIface) *Context {
	ctxes.Lock()
	defer ctxes.Unlock()

	ctxes.last++
	h := ctxes.last

	ctx := &Context{
		h: h, impl: impl,
	}
	ctxes.byHandle[h] = ctx
	return ctx
}

func getContext(h Handle) *Context {
	ctxes.RLock()
	ctx := ctxes.byHandle[h]
	ctxes.RUnlock()
	return ctx
}

// Context is a UAST context that tracks all the resources associated with a given UAST.
type Context struct {
	lasth uint64 // atomic

	h    Handle
	impl NodeIface

	mu    sync.RWMutex
	last  error
	xpath query.Interface
	iters map[Handle]*Iterator
}

// next allocates a new handle in this context. It can be used for any object that belongs to this context.
func (c *Context) next() Handle {
	h := atomic.AddUint64(&c.lasth, 1)
	return Handle(h)
}

// Handle returns an opaque handle of this context.
func (c *Context) Handle() Handle {
	if c == nil {
		return 0
	}
	return c.h
}

// Error returns the last error encountered in this context.
func (c *Context) Error() error {
	if c == nil {
		return errors.New("uast context is nil")
	}
	c.mu.RLock()
	err := c.last
	c.mu.RUnlock()
	return err
}

// free releases all resources associated with this context.
func (c *Context) free() {
	if c == nil {
		return
	}
	ctxes.Lock()
	delete(ctxes.byHandle, c.h)
	ctxes.Unlock()
	c.impl.Free()
	c.iters = nil
}

// Iterator is a UAST node iterator.
type Iterator struct {
	c  *Context
	h  Handle
	it query.Iterator

	skipVals bool
}

// Handle returns a handle for this iterator in the parent Context.
func (it *Iterator) Handle() Handle {
	if it == nil {
		return 0
	}
	return it.h
}

var (
	posType  = uast.TypeOf(uast.Position{})
	possType = uast.TypeOf(uast.Positions{})
)

// Next returns the next node or nil if there are no more nodes to enumerate.
func (it *Iterator) Next() Node {
	if it == nil || it.it == nil {
		return nil
	}
	for {
		if !it.it.Next() {
			return nil
		}
		n := it.it.Node()
		if it.skipVals {
			if n == nil || n.Kind().In(nodes.KindsValues|nodes.KindNil) {
				continue
			} else if obj, ok := n.(nodes.ExternalObject); ok {
				switch uast.TypeOf(obj) {
				case posType, possType:
					continue
				}
			}
		}
		return it.c.toNode(n)
	}
}

// Close releases all resources associated with this iterator.
func (it *Iterator) Close() error {
	if it == nil || it.c == nil {
		return nil
	}
	delete(it.c.iters, it.h)
	it.it = nil
	return nil
}

// setError sets the last error for this context.
func (c *Context) setError(err error) {
	if c.last == nil {
		c.last = err
	}
}

// toNode casts an external node to a node backed by the context.
func (c *Context) toNode(n nodes.External) Node {
	if n == nil {
		return nil
	} else if nd, ok := n.(Node); ok {
		return nd
	}
	if nd, ok := c.impl.(*cNodes); ok {
		return nd.NewValue(n.(nodes.Value))
	}
	// TODO: find a better way to convert these nodes
	return c.impl.(*goNodes).toNode(n.(nodes.Node))
}

// asNode casts a handle to a node backed by the context.
func (c *Context) asNode(h Handle) Node {
	return c.impl.AsNode(h)
}

// AsIterator casts the handle to an iterator or returns nil in case the handle is invalid.
func (c *Context) AsIterator(h Handle) *Iterator {
	return c.iters[h]
}

// NewIterator wraps an a given iterator and binds it to a new handle in this context. Caller should free an iterator.
func (c *Context) NewIterator(it query.Iterator, vals bool) *Iterator {
	h := c.next()
	cit := &Iterator{c: c, h: h, it: it, skipVals: !vals}
	if c.iters == nil {
		c.iters = make(map[Handle]*Iterator)
	}
	c.iters[h] = cit
	return cit
}

// Iterate enumerates all the nodes starting from the root in the specified order. Caller should free an iterator.
func (c *Context) Iterate(root Node, order query.IterOrder) *Iterator {
	it := query.NewIterator(root, order)
	return c.NewIterator(it, false)
}

// Filter executes an XPath query for a given root node and returns an iterator for results. Caller should free an iterator.
func (c *Context) Filter(root Node, query string) (*Iterator, error) {
	if c.xpath == nil {
		c.xpath = xpath.New()
	}
	it, err := c.xpath.Execute(root, query)
	if err != nil {
		c.setError(err)
		return nil, err
	}
	return c.NewIterator(it, true), nil
}

// loadNode loads a given node implementation to Go memory, either by type assertion or by copying memory.
func loadNode(n Node) (nodes.Node, error) {
	if n == nil {
		return nil, nil
	}
	if nd, ok := n.(Native); ok {
		return nd.Native(), nil
	}
	return nodes.ToNode(n, nil)
}
