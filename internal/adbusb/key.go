package adbusb

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
)

// LoadKey reads our RSA key from dir/adbkey, making one the first time.
// The phone remembers this key once the user taps "Always allow".
func LoadKey(dir string) (*rsa.PrivateKey, error) {
	p := filepath.Join(dir, "adbkey")
	if b, err := os.ReadFile(p); err == nil {
		if blk, _ := pem.Decode(b); blk != nil {
			if k, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err == nil {
				if rk, ok := k.(*rsa.PrivateKey); ok {
					return rk, nil
				}
			}
		}
	}
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(dir, 0o700)
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return k, nil
}

// PublicKey encodes a key the way adbd expects (Android's RSAPublicKey
// struct, base64): word count, n0inv, modulus and R^2 as little-endian
// 32-bit words, then the exponent.
func PublicKey(pub *rsa.PublicKey) string {
	const words = 2048 / 32
	n := pub.N
	r32 := new(big.Int).Lsh(big.NewInt(1), 32)
	n0 := new(big.Int).Mod(n, r32)
	n0inv := new(big.Int).ModInverse(n0, r32)
	n0inv.Sub(r32, n0inv) // -1/n[0] mod 2^32
	rr := new(big.Int).Lsh(big.NewInt(1), 2048*2)
	rr.Mod(rr, n)

	buf := make([]byte, 4+4+words*4+words*4+4)
	binary.LittleEndian.PutUint32(buf[0:], words)
	binary.LittleEndian.PutUint32(buf[4:], uint32(n0inv.Uint64()))
	putWords(buf[8:], n, words)
	putWords(buf[8+words*4:], rr, words)
	binary.LittleEndian.PutUint32(buf[8+words*8:], uint32(pub.E))
	return base64.StdEncoding.EncodeToString(buf)
}

func putWords(dst []byte, x *big.Int, words int) {
	be := x.FillBytes(make([]byte, words*4)) // big-endian
	for i := 0; i < words; i++ {
		// word i (little-endian order) = bytes from the end
		w := binary.BigEndian.Uint32(be[len(be)-4*(i+1):])
		binary.LittleEndian.PutUint32(dst[4*i:], w)
	}
}
