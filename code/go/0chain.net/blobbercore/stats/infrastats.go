package stats

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

const (
	HealthDataKey                 = contextKey("health")
	FailedChallengeRequestDataKey = contextKey("fcrd")
	AllocationListRequestDataKey  = contextKey("alrd")
)

type InfraStats struct {
	CPUs               int
	NumberOfGoroutines int
	HeapAlloc          int64
	HeapSys            int64
	ActiveOnChain      string
}
