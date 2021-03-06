package dal

import (
	"crypto/sha1"
	"strconv"
	"strings"
	"time"

	"github.com/coyove/iis/dal/storage"
)

type KeyValueOp interface {
	Get(string) ([]byte, error)
	WeakGet(string) ([]byte, error)
	Set(string, []byte) error
	SetGlobalCache(*storage.GlobalCache)
}

func incdec(a *int32, b *int, inc bool) {
	if a != nil && inc {
		*a++
	} else if a != nil && !inc {
		if *a--; *a < 0 {
			*a = 0
		}
	} else if b != nil && inc {
		*b++
	} else if b != nil && !inc {
		if *b--; *b < 0 {
			*b = 0
		}
	}
}

func makeFollowID(from, to string) string {
	h := sha1.Sum([]byte(to))
	return "u/" + from + "/follow/" + strconv.Itoa(int(h[0]))
}

func makeFollowerAcceptanceID(from, to string) string {
	return "u/" + from + "/accept-follow/" + to
}

func makeFollowedID(from, to string) string {
	return "u/" + from + "/followed/" + to
}

func makeBlockID(from, to string) string {
	return "u/" + from + "/block/" + to
}

func makeLikeID(from, to string) string {
	return "u/" + from + "/like/" + to
}

func makeCheckpointID(from string, t time.Time) string {
	return "u/" + from + "/checkpoint/" + t.Format("2006-01")
}

func lastElemInCompID(id string) string {
	return id[strings.LastIndex(id, "/")+1:]
}

func atoi64(a string) int64 {
	v, _ := strconv.ParseInt(a, 10, 64)
	return v
}

func atob(a string) bool {
	v, _ := strconv.ParseBool(a)
	return v
}
