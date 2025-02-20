package hnswgo

// #cgo LDFLAGS: -L${SRCDIR} -lhnsw -lm
// #include <stdlib.h>
// #include "hnsw_wrapper.h"
// HNSW initHNSW(int dim, unsigned long int max_elements, int M, int ef_construction, int rand_seed, char stype);
// HNSW loadHNSW(char *location, int dim, char stype);
// void addPoint(HNSW index, float *vec, unsigned long int label);
// int searchKnn(HNSW index, float *vec, int N, unsigned long int *label, float *dist);
// void setEf(HNSW index, int ef);
import "C"
import (
	"math"
	"runtime"
	"sync"
	"unsafe"
)

type HNSW struct {
	index     C.HNSW
	spaceType string
	dim       int
	normalize bool
}

// New make a hnsw graph
func New(dim, M, efConstruction, randSeed int, maxElements uint32, spaceType string) *HNSW {
	var hnsw HNSW
	hnsw.dim = dim
	hnsw.spaceType = spaceType
	if spaceType == "ip" {
		hnsw.index = C.initHNSW(C.int(dim), C.ulong(maxElements), C.int(M), C.int(efConstruction), C.int(randSeed), C.char('i'))
	} else if spaceType == "cosine" {
		hnsw.normalize = true
		hnsw.index = C.initHNSW(C.int(dim), C.ulong(maxElements), C.int(M), C.int(efConstruction), C.int(randSeed), C.char('i'))
	} else {
		hnsw.index = C.initHNSW(C.int(dim), C.ulong(maxElements), C.int(M), C.int(efConstruction), C.int(randSeed), C.char('l'))
	}
	return &hnsw
}

// Load load a hnsw graph
func Load(location string, dim int, spaceType string) *HNSW {
	var hnsw HNSW
	hnsw.dim = dim
	hnsw.spaceType = spaceType

	pLocation := C.CString(location)
	if spaceType == "ip" {
		hnsw.index = C.loadHNSW(pLocation, C.int(dim), C.char('i'))
	} else if spaceType == "cosine" {
		hnsw.normalize = true
		hnsw.index = C.loadHNSW(pLocation, C.int(dim), C.char('i'))
	} else {
		hnsw.index = C.loadHNSW(pLocation, C.int(dim), C.char('l'))
	}
	C.free(unsafe.Pointer(pLocation))
	return &hnsw
}

// Unload TODO Test for release the graph memory
func (h *HNSW) Unload() bool {
	if h.index == nil {
		return false
	}
	C.free(unsafe.Pointer(h.index))
	h.index = nil
	// Free memory ASAP, but need to check the memory usage
	runtime.GC()
	return true
}

// Save save graph node on graph
func (h *HNSW) Save(location string) bool {
	if h.index == nil {
		return false
	}
	pLocation := C.CString(location)
	C.saveHNSW(h.index, pLocation)
	C.free(unsafe.Pointer(pLocation))
	return true
}

// normalizeVector normalize vector
func normalizeVector(vector []float32) []float32 {
	var norm float32
	for i := 0; i < len(vector); i++ {
		norm += vector[i] * vector[i]
	}
	norm = 1.0 / (float32(math.Sqrt(float64(norm))) + 1e-15)
	for i := 0; i < len(vector); i++ {
		vector[i] = vector[i] * norm
	}
	return vector
}

// AddPoint add a point on graph
func (h *HNSW) AddPoint(vector []float32, label uint32) bool {
	if h.index == nil {
		return false
	}
	if h.normalize {
		vector = normalizeVector(vector)
	}
	C.addPoint(h.index, (*C.float)(unsafe.Pointer(&vector[0])), C.ulong(label))
	return true
}

// AddBatchPoints add some points on graph
func (h *HNSW) AddBatchPoints(vectors [][]float32, labels []uint32, coroutines int) bool {
	if len(vectors) != len(labels) || coroutines < 1 {
		return false
	}

	b := len(vectors) / coroutines
	var wg sync.WaitGroup
	for i := 0; i < coroutines; i++ {
		wg.Add(1)

		end := (i + 1) * b
		if i == coroutines-1 && len(vectors) > end {
			end = len(vectors)
		}
		go func(thisVectors [][]float32, thisLabels []uint32) {
			defer wg.Done()
			for j := 0; j < len(thisVectors); j++ {
				h.AddPoint(thisVectors[j], thisLabels[j])
			}
		}(vectors[i*b:end], labels[i*b:end])
	}

	wg.Wait()
	return true
}

// SearchKNN search points on graph with knn-algorithm
func (h *HNSW) SearchKNN(vector []float32, N int) ([]uint32, []float32) {
	if h.index == nil {
		return nil, nil
	}
	Clabel := make([]C.ulong, N, N)
	Cdist := make([]C.float, N, N)
	if h.normalize {
		vector = normalizeVector(vector)
	}
	numResult := int(C.searchKnn(h.index, (*C.float)(unsafe.Pointer(&vector[0])), C.int(N), &Clabel[0], &Cdist[0]))
	labels := make([]uint32, N)
	dists := make([]float32, N)
	for i := 0; i < numResult; i++ {
		labels[i] = uint32(Clabel[i])
		dists[i] = float32(Cdist[i])
	}
	return labels[:numResult], dists[:numResult]
}

func (h *HNSW) SearchBatchKNN(vectors [][]float32, N, coroutines int) ([][]uint32, [][]float32) {
	var lock sync.Mutex
	labelList := make([][]uint32, len(vectors))
	distList := make([][]float32, len(vectors))

	b := len(vectors) / coroutines
	var wg sync.WaitGroup
	for i := 0; i < coroutines; i++ {
		wg.Add(1)

		end := (i + 1) * b
		if i == coroutines-1 && len(vectors) > end {
			end = len(vectors)
		}
		go func(i int) {
			defer wg.Done()
			for j := i * b; j < end; j++ {
				labels, dist := h.SearchKNN(vectors[j], N)
				lock.Lock()
				labelList[j] = labels
				distList[j] = dist
				lock.Unlock()
			}
		}(i)
	}
	wg.Wait()
	return labelList, distList
}

// SetEf set ef argument on graph
func (h *HNSW) SetEf(ef int) {
	if h.index == nil {
		return
	}
	C.setEf(h.index, C.int(ef))
}

func (h *HNSW) SetNormalize(isNeedNormalize bool) {
	h.normalize = isNeedNormalize
}

func (h *HNSW) Free() {
	C.freeHNSW(h.index)
}
