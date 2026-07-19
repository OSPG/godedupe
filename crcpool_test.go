package main

import (
	"hash"
	"hash/crc64"
	"sync"
	"testing"
)

var (
	text             = []byte("something to write")
	benchmarkCRCPool = &sync.Pool{
		New: func() interface{} {
			return crc64.New(crc64.MakeTable(crc64.ECMA))
		},
	}
)

func BenchmarkWithPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := benchmarkCRCPool.Get().(hash.Hash64)
		h.Write(text)
		h.Sum64()
		h.Reset()
		benchmarkCRCPool.Put(h)
	}
}

func BenchmarkWithoutPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := crc64.New(crc64.MakeTable(crc64.ECMA))
		h.Write(text)
		h.Sum64()
	}
}
