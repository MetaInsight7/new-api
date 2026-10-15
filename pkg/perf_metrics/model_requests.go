package perfmetrics

import (
	"sort"
	"sync"
	"time"
)

type modelRequestKey struct {
	model  string
	status string
}

type modelRequestCounter struct {
	count       int64
	lastUpdated time.Time
}

type modelRequestSnapshot struct {
	modelRequestKey
	count int64
}

var modelRequests = struct {
	sync.Mutex
	values map[modelRequestKey]modelRequestCounter
}{values: make(map[modelRequestKey]modelRequestCounter)}

func recordModelRequest(sample Sample) {
	status := "error"
	if sample.Success {
		status = "success"
	}
	key := modelRequestKey{model: sample.Model, status: status}
	modelRequests.Lock()
	counter := modelRequests.values[key]
	counter.count++
	counter.lastUpdated = time.Now()
	modelRequests.values[key] = counter
	modelRequests.Unlock()
}

func snapshotModelRequests(cutoff time.Time) []modelRequestSnapshot {
	modelRequests.Lock()
	defer modelRequests.Unlock()
	snapshots := make([]modelRequestSnapshot, 0, len(modelRequests.values))
	for key, counter := range modelRequests.values {
		if counter.lastUpdated.Before(cutoff) {
			delete(modelRequests.values, key)
			continue
		}
		snapshots = append(snapshots, modelRequestSnapshot{modelRequestKey: key, count: counter.count})
	}
	sort.Slice(snapshots, func(left, right int) bool {
		if snapshots[left].model != snapshots[right].model {
			return snapshots[left].model < snapshots[right].model
		}
		return snapshots[left].status < snapshots[right].status
	})
	return snapshots
}
