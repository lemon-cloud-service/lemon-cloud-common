package lccu_strings

import (
	"fmt"
	"github.com/satori/go.uuid"
)

func RandomUUIDString() string {
	return fmt.Sprintf("%s", uuid.NewV4())
}
