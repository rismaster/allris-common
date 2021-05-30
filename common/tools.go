package common

import (
	"crypto/md5"
	"fmt"
)

func Md5HashStr(data string) string {
	dataHash := []byte(data)
	return fmt.Sprintf("%x", md5.Sum(dataHash))
}

func Md5HashB(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
