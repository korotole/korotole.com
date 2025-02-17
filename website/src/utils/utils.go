package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"time"
)

func GetSHA256(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func GetTimestamp() int64 {
	return time.Now().UnixNano()
}

func GetBaseDir() string {
	ex, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Dir(ex)
}
