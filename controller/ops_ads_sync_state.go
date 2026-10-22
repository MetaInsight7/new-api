package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const opsAdsSyncRetryInterval = 5 * time.Minute

type opsAdsSyncResult struct {
	OK        bool
	LastError string
	LastTry   int64
}

var (
	opsAdsSyncMu         sync.Mutex
	opsAdsSyncInFlight   bool
	opsAdsSyncLastTry    time.Time
	opsAdsSyncLastResult opsAdsSyncResult
	opsSyncAdsSpendFunc  = opsSyncAdsSpend
	opsSyncAdsDailyFunc  = opsSyncAdsDaily
)

// ensureAdsSyncStarted keeps external Google Ads requests off report request
// paths. Both syncs retain their own freshness checks; this coordinator only
// prevents duplicate attempts in one process and exposes a sanitized result.
func ensureAdsSyncStarted() {
	if _, configured := opsAdsCredsFromEnv(); !configured {
		return
	}

	opsAdsSyncMu.Lock()
	if opsAdsSyncInFlight || time.Since(opsAdsSyncLastTry) < opsAdsSyncRetryInterval {
		opsAdsSyncMu.Unlock()
		return
	}
	opsAdsSyncInFlight = true
	opsAdsSyncLastTry = time.Now()
	lastTry := opsAdsSyncLastTry.Unix()
	opsAdsSyncMu.Unlock()

	go func() {
		spendErr := opsSyncAdsSpendFunc()
		dailyErr := opsSyncAdsDailyFunc()
		lastError := ""
		switch {
		case spendErr != nil && dailyErr != nil:
			lastError = "ads spend and daily synchronization failed"
		case spendErr != nil:
			lastError = "ads spend synchronization failed"
		case dailyErr != nil:
			lastError = "ads daily synchronization failed"
		}
		if spendErr != nil {
			common.SysError(fmt.Sprintf("ops ads spend sync: %v", spendErr))
		}
		if dailyErr != nil {
			common.SysError(fmt.Sprintf("ops ads daily sync: %v", dailyErr))
		}

		opsAdsSyncMu.Lock()
		opsAdsSyncLastResult = opsAdsSyncResult{OK: lastError == "", LastError: lastError, LastTry: lastTry}
		opsAdsSyncInFlight = false
		opsAdsSyncMu.Unlock()
	}()
}

func currentAdsSyncError() string {
	opsAdsSyncMu.Lock()
	defer opsAdsSyncMu.Unlock()
	if opsAdsSyncLastResult.LastTry == 0 || opsAdsSyncLastResult.OK {
		return ""
	}
	return opsAdsSyncLastResult.LastError
}
