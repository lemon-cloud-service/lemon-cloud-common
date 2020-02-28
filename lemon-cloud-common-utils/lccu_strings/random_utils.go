package lccu_strings

import (
	"fmt"
	"github.com/satori/go.uuid"
	"strings"
)

func RandomUUIDString() string {
	return fmt.Sprintf("%s", uuid.NewV4())
}

func RandomUUIDStringNoLine() string {
	return strings.ReplaceAll(fmt.Sprintf("%s", uuid.NewV4()), "-", "")
}
