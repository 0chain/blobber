package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDiskCheckConfig sets DiskWriteThreshold and restores it after the test.
func setupDiskCheckConfig(t *testing.T, threshold int) {
	t.Helper()
	orig := config.Configuration.DiskWriteThreshold
	config.Configuration.DiskWriteThreshold = threshold
	t.Cleanup(func() { config.Configuration.DiskWriteThreshold = orig })
}

// fakeDiskPct replaces diskUsedPctFn for the duration of a test.
func fakeDiskPct(t *testing.T, pct int, err error) {
	t.Helper()
	orig := diskUsedPctFn
	diskUsedPctFn = func() (int, error) { return pct, err }
	t.Cleanup(func() { diskUsedPctFn = orig })
}

// okHandler is a trivial next-handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestWithDiskSpaceCheck_ThresholdZeroDisabled(t *testing.T) {
	setupDiskCheckConfig(t, 0)
	fakeDiskPct(t, 99, nil) // even 99% used must be ignored when threshold=0

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWithDiskSpaceCheck_BelowThreshold(t *testing.T) {
	setupDiskCheckConfig(t, 90)
	fakeDiskPct(t, 89, nil)

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWithDiskSpaceCheck_AtThreshold_Rejects(t *testing.T) {
	setupDiskCheckConfig(t, 90)
	fakeDiskPct(t, 90, nil)

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	require.Equal(t, http.StatusInsufficientStorage, rr.Code)
	assert.Contains(t, rr.Body.String(), "disk_full")
}

func TestWithDiskSpaceCheck_AboveThreshold_Rejects(t *testing.T) {
	setupDiskCheckConfig(t, 90)
	fakeDiskPct(t, 95, nil)

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	require.Equal(t, http.StatusInsufficientStorage, rr.Code)
	assert.Contains(t, rr.Body.String(), "disk_full")
}

func TestWithDiskSpaceCheck_StatError_FailOpen(t *testing.T) {
	// A stat failure must not block uploads — fail open.
	setupDiskCheckConfig(t, 90)
	fakeDiskPct(t, 0, errors.New("statfs: no such file or directory"))

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWithDiskSpaceCheck_ResponseContentType(t *testing.T) {
	setupDiskCheckConfig(t, 90)
	fakeDiskPct(t, 91, nil)

	rr := httptest.NewRecorder()
	WithDiskSpaceCheck(okHandler)(rr, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}
