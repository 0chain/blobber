package common

import (
	"time"
)

//DateTimeFormat - the format in which the date time fields should be displayed in the UI
var DateTimeFormat = "2006-01-02T15:04:05+00:00"

/*Timestamp - just a wrapper to control the json encoding */
type Timestamp int64

/*Now - current datetime */
func Now() Timestamp {
	return Timestamp(time.Now().Unix())
}

/*Within ensures a given timestamp is within certain number of seconds */
func Within(ts, seconds int64) bool {
	now := time.Now().Unix()
	return now > ts-seconds && now < ts+seconds
}

//ToTime - converts the common.Timestamp to time.Time
func ToTime(ts Timestamp) time.Time {
	return time.Unix(int64(ts), 0)
}

// ValidateLt will validate timestamp if it is within (currentTime - duration) and currentTime
func ValidateLt(ts Timestamp, d time.Duration) bool {
	now := Now()
	return ts < now && ts > (now-Timestamp(d.Seconds()))
}
