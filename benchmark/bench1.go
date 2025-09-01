package main

import (
	"flag"
	"fmt"
	"mss"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

type Type1 struct{ X int }
type Type2 struct{ Y [5]rune }
type Type3 struct {
	Z, Z2 *int
}

var (
	nItems      = flag.Int("n", 10000, "rows per iteration")
	iters       = flag.Int("iters", 50, "iterations of each benchmark")
	verify      = flag.Bool("verify", false, "do correctness checks during reads")
	cpuprof     = flag.String("cpuprofile", "", "write CPU profile to file")
	memprof     = flag.String("memprofile", "", "write heap profile to file (at end)")
	gomax       = flag.Int("gomaxprocs", 0, "set GOMAXPROCS (0 = keep default)")
	warmupIters = flag.Int("warmup", 2, "warmup iterations (not timed)")
)

func main() {
	flag.Parse()
	if *gomax > 0 {
		runtime.GOMAXPROCS(*gomax)
	}
	if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			panic(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}

	// Warm up allocator/caches so first-iter effects don't skew.
	runDynamic(1, *warmupIters, false)
	runDefault(1, *warmupIters, false)

	// Timed runs.
	addDyn, readDyn := runDynamic(*nItems, *iters, *verify)
	addDef, readDef := runDefault(*nItems, *iters, *verify)

	ops := float64((*nItems) * (*iters))
	perOp := func(d time.Duration) float64 {
		// d is already in nanoseconds; divide by #ops to get ns/op.
		return float64(d) / ops
	}

	fmt.Printf("\n== ArrayofStructs ==\n")
	fmt.Printf("Add:   %8.2f ns/op (%v total)\n", perOp(addDyn), addDyn)
	fmt.Printf("Read:  %8.2f ns/op (%v total)\n", perOp(readDyn), readDyn)
	fmt.Printf("Total: %8.2f ns/op (%v total)\n", perOp(addDyn+readDyn), addDyn+readDyn)

	fmt.Printf("\n== Plain Go slice ==\n")
	fmt.Printf("Add:   %8.2f ns/op (%v total)\n", perOp(addDef), addDef)
	fmt.Printf("Read:  %8.2f ns/op (%v total)\n", perOp(readDef), readDef)
	fmt.Printf("Total: %8.2f ns/op (%v total)\n", perOp(addDef+readDef), addDef+readDef)

	if *memprof != "" {
		f, err := os.Create(*memprof)
		if err != nil {
			panic(err)
		}
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(err)
		}
		_ = f.Close()
	}
}

func runDynamic(n, iters int, doVerify bool) (addDur, readDur time.Duration) {
	testint := new(int)
	*testint = 3000
	for nIter := 0; nIter < iters; nIter++ {
		var arr mss.MixedStructSlice
		arr.AddComponent(Type1{10})
		arr.AddComponent(Type2{})
		arr.AddComponent(Type3{testint, testint})
		arr.Build()

		runeList := [5]rune{'1', '2', '3', '4', '5'}

		start := time.Now()
		for i := 0; i < n; i++ {
			arr.Add(Type1{i}, Type2{runeList}, Type3{testint, testint})
		}
		addDur += time.Since(start)

		// Resolve column indices once for this T (avoids per-read type scanning).
		col1 := mss.ColOf[Type1](&arr)
		col2 := mss.ColOf[Type2](&arr)
		col3 := mss.ColOf[Type3](&arr)

		start = time.Now()
		for x := 0; x < 50; x++ {

			for i := 0; i < n; i++ {
				out1 := mss.IndexRowCol[Type1](&arr, i, col1)
				out2 := mss.IndexRowCol[Type2](&arr, i, col2)
				out3 := mss.IndexRowCol[Type3](&arr, i, col3)
				*testint++
				out1.X = i
				if doVerify {
					if out2.Y[4] != '5' {
						panic("AOS: wrong rune stored")
					}
					if *out3.Z != *testint {
						panic("AOS: pointer not preserved")
					}
				}
			}
		}
		readDur += time.Since(start)
	}
	return
}

func runDefault(n, iters int, doVerify bool) (addDur, readDur time.Duration) {
	testint := new(int)
	*testint = 3000

	type DefaultStruct struct {
		Type1
		Type2
		Type3
	}

	for nIter := 0; nIter < iters; nIter++ {
		s := make([]DefaultStruct, 0, 0)

		start := time.Now()
		for i := 0; i < n; i++ {
			t1 := Type1{i}
			t2 := Type2{[5]rune{'1', '2', '3', '4', '5'}}
			t3 := Type3{testint, testint}
			s = append(s, DefaultStruct{t1, t2, t3})
		}
		addDur += time.Since(start)

		start = time.Now()
		for x := 0; x < 50; x++ {

			for i := 0; i < n; i++ {
				out1 := &s[i].Type1
				out2 := &s[i].Type2
				out3 := &s[i].Type3
				*testint++
				out1.X = i
				if doVerify {
					if out2.Y[4] != '5' {
						panic("plain: wrong rune")
					}
					if *out3.Z != *testint {
						panic("plain: pointer not preserved")
					}
				}
			}
		}
		readDur += time.Since(start)
	}
	return
}
