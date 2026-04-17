package auction

import "time"

func nowUnixNano() int64 {
	return time.Now().UnixNano()
}
