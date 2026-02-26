package bench

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"crypto/rand"

	"git.wyat.me/git-storage/object"
	"git.wyat.me/git-storage/store"
)

type Size struct {
	Name  string
	Bytes int
}

var Sizes = []Size{
	{"small (1KB)", 1024},
	{"medium (100KB)", 100 * 1024},
	{"large (1MB)", 1024 * 1024},
}

type OperationResult struct {
	P50 time.Duration
	P99 time.Duration
	OpsPerSec float64
}

type SizeResult struct {
	Size        Size
	Put         OperationResult
	Get         OperationResult
	Exists      OperationResult
	ConcurrentPut OperationResult
}

type BackendResult struct {
	Backend string
	Results []SizeResult
	Error   string
}

type RunResult struct {
	Timestamp time.Time
	Backends  []BackendResult
}

const iterations = 100

func randomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func measure(n int, fn func() error) (OperationResult, error) {
	latencies := make([]time.Duration, 0, n)

	for range n {
		start := time.Now()
		if err := fn(); err != nil {
			return OperationResult{}, err
		}
		latencies = append(latencies, time.Since(start))
	}

	// sort for percentiles
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[j] < latencies[i] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	p50 := latencies[len(latencies)*50/100]
	p99 := latencies[len(latencies)*99/100]
	total := time.Duration(0)
	for _, l := range latencies {
		total += l
	}
	opsPerSec := float64(n) / total.Seconds()

	return OperationResult{P50: p50, P99: p99, OpsPerSec: opsPerSec}, nil
}

func measureConcurrent(n int, fn func() error) (OperationResult, error) {
	workers := runtime.NumCPU()
	latencies := make([]time.Duration, n)
	errCh := make(chan error, n)
	jobs := make(chan int, n)

	for i := range n {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				start := time.Now()
				if err := fn(); err != nil {
					errCh <- err
					return
				}
				latencies[i] = time.Since(start)
			}
		})
	}
	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return OperationResult{}, err
	}

	// sort for percentiles
	for i := range latencies {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[j] < latencies[i] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	p50 := latencies[len(latencies)*50/100]
	p99 := latencies[len(latencies)*99/100]
	total := time.Duration(0)
	for _, l := range latencies {
		total += l
	}
	opsPerSec := float64(n) / total.Seconds()

	return OperationResult{P50: p50, P99: p99, OpsPerSec: opsPerSec}, nil
}

func RunBackend(name string, s store.ObjectStore) BackendResult {
	result := BackendResult{Backend: name}

	for _, size := range Sizes {
		sr := SizeResult{Size: size}
		data := randomData(size.Bytes)

		// pre-populate SHAs for Get/Exists
		shas := make([]string, iterations)
		for i := range iterations {
			obj := &object.Object{Type: object.TypeBlob, Data: randomData(size.Bytes)}
			sha, err := s.Put(obj)
			if err != nil {
				result.Error = fmt.Sprintf("setup Put failed: %v", err)
				return result
			}
			shas[i] = sha
		}

		// Put
		putResult, err := measure(iterations, func() error {
			obj := &object.Object{Type: object.TypeBlob, Data: data}
			_, err := s.Put(obj)
			return err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Put benchmark failed: %v", err)
			return result
		}
		sr.Put = putResult

		// Get
		i := 0
		getResult, err := measure(iterations, func() error {
			_, err := s.Get(shas[i%len(shas)])
			i++
			return err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Get benchmark failed: %v", err)
			return result
		}
		sr.Get = getResult

		// Exists
		j := 0
		existsResult, err := measure(iterations, func() error {
			_, err := s.Exists(shas[j%len(shas)])
			j++
			return err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Exists benchmark failed: %v", err)
			return result
		}
		sr.Exists = existsResult

		// Concurrent Put
		concResult, err := measureConcurrent(iterations, func() error {
			obj := &object.Object{Type: object.TypeBlob, Data: randomData(size.Bytes)}
			_, err := s.Put(obj)
			return err
		})
		if err != nil {
			result.Error = fmt.Sprintf("Concurrent Put benchmark failed: %v", err)
			return result
		}
		sr.ConcurrentPut = concResult

		result.Results = append(result.Results, sr)
	}

	return result
}

