package sha1

import (
	"crypto/sha1"
)

type Sha1Hasher struct{}

func NewHasher() Sha1Hasher {
	return Sha1Hasher{}
}

func (sh Sha1Hasher) Hash(data []byte) []byte {
	hash := sha1.Sum(data)
	return hash[0:sh.Size()]
}

func (sh Sha1Hasher) Size() int {
	return sha1.Size
}
