package main

/*
#include "uast.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/bblfsh/sdk/v3/uast/transformer/positioner"
)

const (
	sizeUastSourceIndex = unsafe.Sizeof(C.UastSourceIndex{})
)

var srcInd = struct {
	sync.RWMutex
	last     Handle
	byHandle map[Handle]*sourceIndex
}{
	byHandle: make(map[Handle]*sourceIndex),
}

type sourceIndex struct {
	idx *positioner.Index
	err error
}

func (idx *sourceIndex) setError(err error) {
	idx.err = err
}

func (idx *sourceIndex) lastError() error {
	return idx.err
}

// newSourceIndex allocates and populates the UastSourceIndex structure for C code to use.
func newSourceIndex(h Handle) *C.UastSourceIndex {
	v := (*C.UastSourceIndex)(C.malloc(C.size_t(sizeUastSourceIndex)))
	v.handle = C.uintptr_t(h)
	return v
}

//export UastSourceIndexNew
// UastSourceIndexNew creates a positional index for a source file. An index can be used
// to convert byte-based node offsets to Unicode character offsets.
func UastSourceIndexNew(source unsafe.Pointer, size C.size_t) *C.UastSourceIndex {
	index := positioner.NewIndex(cBytes(source, size), &positioner.IndexOptions{
		Unicode: true,
	})
	srcInd.Lock()
	srcInd.last++
	h := srcInd.last
	srcInd.byHandle[h] = &sourceIndex{idx: index}
	srcInd.Unlock()
	return newSourceIndex(h)
}

//export UastSourceIndexFree
// UastSourceIndexFree release all resources associated with an index.
func UastSourceIndexFree(idx *C.UastSourceIndex) {
	if idx == nil {
		return
	}
	h := idx.handle
	C.free(unsafe.Pointer(idx))
	if h == 0 {
		return
	}
	srcInd.Lock()
	delete(srcInd.byHandle, Handle(h))
	srcInd.Unlock()
}

//export UastSourceIndex_LastError
// UastSourceIndex_LastError returns the last error returned by index operations.
func UastSourceIndex_LastError(idx *C.UastSourceIndex) *C.char {
	index := getSourceIndexFrom(idx)
	if index == nil {
		return nil
	}
	err := index.lastError()
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}

//export UastSourceIndex_ClearError
// UastSourceIndex_ClearError clears an error state.
func UastSourceIndex_ClearError(idx *C.UastSourceIndex) {
	if index := getSourceIndexFrom(idx); index != nil {
		index.setError(nil)
	}
}

// getSourceIndexFrom returns an index for a given C.UastSourceIndex pointer. It returns nil if C structure is invalid.
func getSourceIndexFrom(idx *C.UastSourceIndex) *sourceIndex {
	if idx == nil || idx.handle == 0 {
		return nil
	}
	return getSourceIndex(Handle(idx.handle))
}

// getSourceIndex returns an index for a given handle. It returns nil if handle is invalid.
func getSourceIndex(h Handle) *sourceIndex {
	srcInd.RLock()
	idx := srcInd.byHandle[h]
	srcInd.RUnlock()
	return idx
}

// convertOffset is a helper to convert one offset to the other, given a conversion function.
func convertOffset(fnc func(*positioner.Index, int) (int, error),
	idx *C.UastSourceIndex, off C.int) C.int {
	if off < 0 {
		return -1
	}
	index := getSourceIndexFrom(idx)
	if index == nil {
		return -1
	}
	roff, err := fnc(index.idx, int(off))
	if err != nil {
		index.setError(err)
		return -1
	}
	return C.int(roff)
}

// convertToLineCol is a helper to convert one offset to the line-column pair, given a conversion function.
func convertToLineCol(fnc func(*positioner.Index, int) (int, int, error),
	idx *C.UastSourceIndex, off C.int) C.UastLineCol {
	if off < 0 {
		return C.UastLineCol{-1, -1}
	}
	index := getSourceIndexFrom(idx)
	if index == nil {
		return C.UastLineCol{-1, -1}
	}
	line, col, err := fnc(index.idx, int(off))
	if err != nil {
		index.setError(err)
		return C.UastLineCol{-1, -1}
	}
	var pos C.UastLineCol
	pos.line = C.int(line)
	pos.col = C.int(col)
	return pos
}

//export UastSourceIndex_FromLineCol
// UastSourceIndex_FromLineCol converts one-based line-column pair (in bytes) in the indexed
// source file to a zero-based byte offset. It return -1 in case of failure.
func UastSourceIndex_FromLineCol(idx *C.UastSourceIndex, line, col C.int) C.int {
	if line < 0 || col < 0 {
		return -1
	}
	index := getSourceIndexFrom(idx)
	if index == nil {
		return -1
	}
	off, err := index.idx.Offset(int(line), int(col))
	if err != nil {
		index.setError(err)
		return -1
	}
	return C.int(off)
}

//export UastSourceIndex_ToLineCol
// UastSourceIndex_ToLineCol converts zero-based byte offset in the indexed source
// file to a one-based line and one-based column pair (in bytes).
// It return a UastLineCol with both elements set to -1 in case of failure.
func UastSourceIndex_ToLineCol(idx *C.UastSourceIndex, off C.int) C.UastLineCol {
	return convertToLineCol((*positioner.Index).LineCol, idx, off)
}

//export UastSourceIndex_FromUnicode
// UastSourceIndex_FromUnicode converts zero-based Unicode character offset in the indexed
// source file to a zero-based byte offset. It return -1 in case of failure.
func UastSourceIndex_FromUnicode(idx *C.UastSourceIndex, off C.int) C.int {
	return convertOffset((*positioner.Index).FromRuneOffset, idx, off)
}

//export UastSourceIndex_ToUnicode
// UastSourceIndex_ToUnicode converts zero-based byte offset in the indexed source file to
// a zero-based Unicode character offset. It return -1 in case of failure.
func UastSourceIndex_ToUnicode(idx *C.UastSourceIndex, off C.int) C.int {
	return convertOffset((*positioner.Index).ToRuneOffset, idx, off)
}

//export UastSourceIndex_FromUTF16
// UastSourceIndex_FromUTF16 converts zero-based UTF-16 code point offset in the indexed
// source file to a zero-based byte offset. It return -1 in case of failure.
func UastSourceIndex_FromUTF16(idx *C.UastSourceIndex, off C.int) C.int {
	return convertOffset((*positioner.Index).FromUTF16Offset, idx, off)
}

//export UastSourceIndex_ToUTF16
// UastSourceIndex_ToUTF16 converts zero-based byte offset in the indexed source file to
// a zero-based UTF-16 code point offset. It return -1 in case of failure.
func UastSourceIndex_ToUTF16(idx *C.UastSourceIndex, off C.int) C.int {
	return convertOffset((*positioner.Index).ToUTF16Offset, idx, off)
}

//export UastSourceIndex_ToUnicodeLineCol
// UastSourceIndex_ToUnicodeLineCol converts zero-based byte offset in the indexed source
// file to a one-based line and one-based column pair (in Unicode characters).
// It return a UastLineCol with both elements set to -1 in case of failure.
func UastSourceIndex_ToUnicodeLineCol(idx *C.UastSourceIndex, off C.int) C.UastLineCol {
	return convertToLineCol((*positioner.Index).ToUnicodeLineCol, idx, off)
}

//export UastSourceIndex_ToUTF16LineCol
// UastSourceIndex_ToUTF16LineCol converts zero-based byte offset in the indexed source
// file to a one-based line and one-based column pair (in UTF-16 code units).
// It return a UastLineCol with both elements set to -1 in case of failure.
func UastSourceIndex_ToUTF16LineCol(idx *C.UastSourceIndex, off C.int) C.UastLineCol {
	return convertToLineCol((*positioner.Index).ToUTF16LineCol, idx, off)
}
