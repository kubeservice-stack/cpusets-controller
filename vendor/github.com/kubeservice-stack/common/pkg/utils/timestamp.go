package utils

import (
	"math/rand"
	"strconv"
	"time"
)

type Timestamp struct {
	time.Time
}

func (t Timestamp) String() string {
	return t.Time.String()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Time is expected in RFC3339 or Unix format.
func (t *Timestamp) UnmarshalJSON(data []byte) (err error) {
	str := string(data)
	i, err := strconv.ParseInt(str, 10, 64)
	if err == nil {
		(*t).Time = time.Unix(i, 0)
	} else {
		(*t).Time, err = time.Parse(`"`+time.RFC3339+`"`, str)
	}
	return
}

// Equal reports whether t and u are equal based on time.Equal
func (t Timestamp) Equal(u Timestamp) bool {
	return t.Time.Equal(u.Time)
}

//ms
func RandSleep(max, min int) {
	if min <= 0 {
		min = 5
	}
	if max <= min {
		max = min + 1
	}
	t := time.Duration(rand.Intn(max-min) + min)
	time.Sleep(t * time.Millisecond)

}

func ToTime(str string) (time.Time, error) {
	timeLayout := "2006-01-02 15:04:05"
	loc, _ := time.LoadLocation("Local")
	return time.ParseInLocation(timeLayout, str, loc)
}
