package main

import (
	"fmt"
	"runtime"
)
const NIL_COMMIT string = "0000000000000000000000000000000000000000"
const SPAN_NOT_SET uint32 = 0
const START_SPAN uint32 = 1

const REF_BRANCH_PREFIX string = "refs/heads/"
const REF_TAG_PREFIX string = "refs/tags/"

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func PrintMemUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	return fmt.Sprintf("Alloc = %v MiB\tTotalAlloc = %v MiB\tSys = %v MiB\tNumGC = %v",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
}