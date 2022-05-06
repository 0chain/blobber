package writemarker

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWriteMarkerEntity_OnChain(t *testing.T) {
	assert.False(t, (&WriteMarkerEntity{Status: Accepted}).OnChain())
	assert.False(t, (&WriteMarkerEntity{Status: Failed}).OnChain())
	assert.True(t, (&WriteMarkerEntity{Status: Committed}).OnChain())
	assert.False(t, (&WriteMarkerEntity{}).OnChain()) // unspecified
}
