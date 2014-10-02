package sha1

import (
	"crypto/sha1"
)

type Sha1Hasher struct{}

func (sh Sha1Hasher) Hash(data []byte) []byte {
	return sha1.Sum(data)[0:sha1.Size]
}

func (sh Sha1Hasher) Size() int {
	return sha1.Size
}
