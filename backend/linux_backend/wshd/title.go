package main

import (
	"os"
	"reflect"
	"unsafe"
)

func setProcTitle(title string) {
	argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
	argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:argv0str.Len]

	for i := 0; i < len(title); i++ {
		argv0[i] = title[i]
	}
}
