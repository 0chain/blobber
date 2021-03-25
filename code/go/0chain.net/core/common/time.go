package common

import (
	"math/rand"
	"time"
)

/*Timestamp - just a wrapper to control the json encoding */
type Timestamp int64

/*Now - current datetime */
func Now() Timestamp {
	return Timestamp(time.Now().Unix())
}

/*Within ensures a given timestamp is within certain number of seconds */
func Within(ts int64, seconds int64) bool {
	now := time.Now().Unix()
	return now > ts-seconds && now < ts+seconds
}

var randGenerator = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:unused,deadcode,varcheck // might be used later?
