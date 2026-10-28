package decorator

import (
	"crypto/md5"  // skipcq: GSC-G501
	"crypto/sha1" // skipcq: GSC-G505
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"

	lua "github.com/krakend/krakend-lua/v3"
)

type hasherType int8

const (
	Fnv1_32 hasherType = iota
	Fnv1_64
	Fnv1_32a
	Fnv1_64a
	Sha1
	Sha224
	Sha256
	Sha384
	Sha512
	Md5
)

func RegisterHashing(b *lua.Binder) {
	t := b.Table("hashing")
	t.Static("fnv1_32", hashingHandler(Fnv1_32))
	t.Static("fnv1_64", hashingHandler(Fnv1_64))
	t.Static("fnv1_32a", hashingHandler(Fnv1_32a))
	t.Static("fnv1_64a", hashingHandler(Fnv1_64a))
	t.Static("sha1", hashingHandler(Sha1))
	t.Static("sha224", hashingHandler(Sha224))
	t.Static("sha256", hashingHandler(Sha256))
	t.Static("sha384", hashingHandler(Sha384))
	t.Static("sha512", hashingHandler(Sha512))
	t.Static("md5", hashingHandler(Md5))
}

func hashingHandler(hasherType hasherType) lua.Handler {
	return func(c *lua.Context) error {
		var hasher hash.Hash
		switch hasherType {
		case Fnv1_32:
			hasher = fnv.New32()
		case Fnv1_32a:
			hasher = fnv.New32a()
		case Fnv1_64:
			hasher = fnv.New64()
		case Fnv1_64a:
			hasher = fnv.New64a()
		case Sha1:
			hasher = sha1.New() // skipcq: GO-S1025
		case Sha224:
			hasher = sha256.New224()
		case Sha256:
			hasher = sha256.New()
		case Sha384:
			hasher = sha512.New384()
		case Sha512:
			hasher = sha512.New()
		case Md5:
			hasher = md5.New() // skipcq: GO-S1023
		default:
			return errors.New("unsupported hasher")
		}

		t := c.Arg(1).String()
		hasher.Write([]byte(t))
		c.Push().String(fmt.Sprintf("%x", hasher.Sum(nil)))

		return nil
	}
}
