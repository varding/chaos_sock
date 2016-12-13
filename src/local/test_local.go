package main

import (
	"testing"
)

func TestSlice(t *testing.T) {
	var buf [64]byte
	p := buf[4:12]
	fmt.Println(cap(p), len(p), p)
	//可以用超出当前size的范围，只要不超过cap就行
	p = p[:12]
	fmt.Println(cap(p), len(p), p)
	//这样只能用12个，不能扩大slice
	p = p[:]
	fmt.Println(cap(p), len(p))
}
