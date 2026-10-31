package store

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// NewItemID returns a fresh item id: "m-" plus 8 lowercase base32 characters (R-MSG-044). Five
// random bytes encode to exactly eight characters without padding.
//
// ponytail: 40 bits and no retry on a primary-key collision; the odds at the scale of one
// project's store are below 1e-8 per insert. Add a retry in Send if a collision is ever seen.
func NewItemID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	return "m-" + strings.ToLower(base32.StdEncoding.EncodeToString(b[:]))
}
