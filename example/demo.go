package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

	hnswgo "github.com/Bing-dwendwen/hnswlib-to-go"
)

func toMegaBytes(bytes uint64) float64 {
	return float64(bytes) / 1024 / 1024
}

func traceMemStats() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var result = make([]float64, 8)
	result[0] = float64(ms.HeapObjects)
	result[1] = toMegaBytes(ms.HeapAlloc)
	result[2] = toMegaBytes(ms.TotalAlloc)
	result[3] = toMegaBytes(ms.HeapSys)
	result[4] = toMegaBytes(ms.HeapIdle)
	result[5] = toMegaBytes(ms.HeapReleased)
	result[6] = toMegaBytes(ms.HeapIdle - ms.HeapReleased)
	result[7] = toMegaBytes(ms.Alloc)

	fmt.Printf("%d\t", time.Now().Unix())
	for _, v := range result {
		fmt.Printf("%.2f\t", v)
	}
	fmt.Printf("\n")
	time.Sleep(2 * time.Second)
}

func randVector(dim int) []float32 {
	vec := make([]float32, dim)
	for j := 0; j < dim; j++ {
		vec[j] = rand.Float32()
	}
	return vec
}

func main() {
	var dim, M, ef int = 128, 32, 300
	// 最大的 elements 数
	var maxElements uint32 = 50000
	// 定义距离 cosine
	var spaceType, indexLocation string = "cosine", "hnsw_demo_index.bin"
	var randomSeed int = 100
	fmt.Println("Before Create HNSW")
	traceMemStats()
	// Init new index
	h := hnswgo.New(dim, M, ef, randomSeed, maxElements, spaceType)
	// Insert 1000 vectors to index. Label Type is uint32
	vectorList := make([][]float32, maxElements)
	ids := make([]uint32, maxElements)
	var i uint32
	for ; i < maxElements; i++ {
		if i%1000 == 0 {
			fmt.Println(i)
		}
		vectorList[i] = randVector(dim)
		ids[i] = i
		//h.AddPoint(randVector(dim), i)
	}

	h.AddBatchPoints(vectorList, ids, 10)

	h.Save(indexLocation)
	h = hnswgo.Load(indexLocation, dim, spaceType)
	// Search vector with maximum 5 NN
	h.SetEf(15)
	searchVector := randVector(dim * 2)
	// Count query time
	startTime := time.Now().UnixNano()
	labels, vectors := h.SearchKNN(searchVector, 5)
	endTime := time.Now().UnixNano()
	fmt.Println(endTime - startTime)
	fmt.Println(labels, vectors)

	labelList, scores := h.SearchBatchKNN(vectorList[:50], 5, 5)
	fmt.Println(labelList, scores)

	fmt.Println("Before Unload")
	traceMemStats()
	h.Unload()
	fmt.Println("After Unload")
	traceMemStats()

	h.Free()
}
