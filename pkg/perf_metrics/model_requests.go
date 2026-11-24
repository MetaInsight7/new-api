package perfmetrics

import (
	"sort"
	"sync"
	"time"
)

type modelOutcomeKey struct {
	model   string
	outcome Outcome
}

type modelOutcomeCounter struct {
	count       int64
	lastUpdated time.Time
}

type modelOutcomeSnapshot struct {
	modelOutcomeKey
	count int64
}

type legacyModelRequestKey struct {
	model  string
	status string
}

type legacyModelRequestSnapshot struct {
	model  string
	status string
	count  int64
}

var modelOutcomes = struct {
	sync.Mutex
	values map[modelOutcomeKey]modelOutcomeCounter
}{values: make(map[modelOutcomeKey]modelOutcomeCounter)}

func recordModelOutcome(sample Sample) {
	key := modelOutcomeKey{model: sample.Model, outcome: sample.Outcome}
	modelOutcomes.Lock()
	counter := modelOutcomes.values[key]
	counter.count++
	counter.lastUpdated = time.Now()
	modelOutcomes.values[key] = counter
	modelOutcomes.Unlock()
}

func snapshotModelOutcomes(cutoff time.Time) []modelOutcomeSnapshot {
	modelOutcomes.Lock()
	defer modelOutcomes.Unlock()
	snapshots := make([]modelOutcomeSnapshot, 0, len(modelOutcomes.values))
	for key, counter := range modelOutcomes.values {
		if counter.lastUpdated.Before(cutoff) {
			delete(modelOutcomes.values, key)
			continue
		}
		snapshots = append(snapshots, modelOutcomeSnapshot{modelOutcomeKey: key, count: counter.count})
	}
	sort.Slice(snapshots, func(left, right int) bool {
		if snapshots[left].model != snapshots[right].model {
			return snapshots[left].model < snapshots[right].model
		}
		return snapshots[left].outcome < snapshots[right].outcome
	})
	return snapshots
}

func legacyModelRequestSnapshots(outcomes []modelOutcomeSnapshot) []legacyModelRequestSnapshot {
	counts := make(map[legacyModelRequestKey]int64)
	for _, snapshot := range outcomes {
		status := "error"
		if snapshot.outcome == OutcomeSuccess {
			status = "success"
		}
		counts[legacyModelRequestKey{model: snapshot.model, status: status}] += snapshot.count
	}
	snapshots := make([]legacyModelRequestSnapshot, 0, len(counts))
	for key, count := range counts {
		snapshots = append(snapshots, legacyModelRequestSnapshot{model: key.model, status: key.status, count: count})
	}
	sort.Slice(snapshots, func(left, right int) bool {
		if snapshots[left].model != snapshots[right].model {
			return snapshots[left].model < snapshots[right].model
		}
		return snapshots[left].status < snapshots[right].status
	})
	return snapshots
}
