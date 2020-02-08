package lccm_marshal

import (
	"fmt"
	"time"
)

type TimeJsonTimeStamp time.Time

func (t TimeJsonTimeStamp) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("%d", time.Time(t).Unix())
	return []byte(stamp), nil
}
