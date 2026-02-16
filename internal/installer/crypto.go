package installer

import (
    "crypto/rand"
    "encoding/hex"
)

func randReadImpl(b []byte) (int, error) {
    return rand.Read(b)
}

func hexEncodeImpl(b []byte) string {
    return hex.EncodeToString(b)
}