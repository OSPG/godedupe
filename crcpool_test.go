package main

import (
	"hash"
	"hash/crc64"
	"testing"
)

var (
	text = []byte("something to write")
)

func BenchmarkWithPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := crcPool.Get().(hash.Hash64)
		h.Write(text)
		h.Reset()
		crcPool.Put(h)
	}
}

func BenchmarkWithoutPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := crc64.New(crc64.MakeTable(crc64.ECMA))
		h.Write(text)
	}
}
