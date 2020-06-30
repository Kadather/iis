package ik

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"

	"github.com/coyove/common/lru"
	"github.com/coyove/iis/common"
	"github.com/coyove/iis/ik/captcha"
	"github.com/coyove/iis/model"
	"github.com/gin-gonic/gin"
)

var (
	Dedup          = lru.NewCache(1024)
	burstableDedup [1024]struct {
		sync.Mutex
		start time.Time
		count int
	}
	maxBurst      = 20
	burstCooldown = 30000
)

func init() {
	go func() {
		// e.g. max 15 requests in 30 seconds, that's 0.5 r/s, so we tick every 2s
		for range time.Tick(time.Duration(burstCooldown/maxBurst) * time.Millisecond) {
			for i := range burstableDedup {
				b := &burstableDedup[i]
				b.Lock()
				if b.count > 1 {
					b.count--
				}
				b.Unlock()
			}
		}
	}()
}

func BAdd(key string) bool {
	b := &burstableDedup[common.Hash32(key)%uint32(len(burstableDedup))]
	b.Lock()
	defer b.Unlock()
	if b.count <= maxBurst {
		b.count++
		return true
	}
	return false
}

func MakeToken(g *gin.Context) (string, string) {
	var x [4]byte
	uuid := MakeUUID(g, &x)
	return uuid, captcha.NewImage(common.Cfg.Key, x[:], 120, 50).PNGBase64()
}

func MakeUUID(g *gin.Context, x *[4]byte) string {
	var p [16]byte
	exp := time.Now().Add(time.Minute * time.Duration(common.Cfg.TokenTTL)).Unix()
	binary.BigEndian.PutUint32(p[:], uint32(exp))

	u, _ := g.Get("user")
	if u == nil {
		copy(p[4:10], g.Request.UserAgent())
	} else {
		copy(p[4:10], u.(*model.User).ID)
	}
	rand.Read(p[10:])

	if x != nil {
		copy((*x)[:], p[10:])
	}

	common.Cfg.Blk.Encrypt(p[:], p[:])
	return hex.EncodeToString(p[:])
}

func ParseToken(g *gin.Context, tok string) (r []byte, ok bool) {
	buf, _ := hex.DecodeString(tok)
	if len(buf) != 16 {
		return
	}
	common.Cfg.Blk.Decrypt(buf, buf)
	exp := binary.BigEndian.Uint32(buf)
	if now := time.Now(); now.After(time.Unix(int64(exp), 0)) ||
		now.Before(time.Unix(int64(exp)-common.Cfg.TokenTTL*60, 0)) {
		return
	}

	tmp := [6]byte{}

	u, _ := g.Get("user")
	if u != nil {
		copy(tmp[:], u.(*model.User).ID)
	} else {
		copy(tmp[:], g.Request.UserAgent())
	}

	ok = bytes.HasPrefix(buf[4:10], tmp[:])
	if ok {
		if _, existed := Dedup.Get(tok); existed {
			return nil, false
		}
		Dedup.Add(tok, true)
	}

	r = buf[10:]
	return
}

func MakeOTT(id string) string {
	if len(id) == 0 {
		return ""
	}

	var nonce [12]byte
	exp := time.Now().Add(time.Second * time.Duration(common.Cfg.IDTokenTTL)).Unix()
	binary.BigEndian.PutUint32(nonce[:], uint32(exp))
	rand.Read(nonce[4:])

	idbuf := make([]byte, len(id), len(id)+48)
	copy(idbuf, id)

	gcm, _ := cipher.NewGCM(common.Cfg.Blk)
	data := gcm.Seal(idbuf[:0], nonce[:], idbuf, nil)
	return base64.URLEncoding.EncodeToString(append(data, nonce[:]...))
}

func ValidateOTT(id, tok string) bool {
	idbuf, _ := base64.URLEncoding.DecodeString(tok)
	if len(idbuf) < 12 {
		return false
	}

	nonce := idbuf[len(idbuf)-12:]
	idbuf = idbuf[:len(idbuf)-12]

	exp := time.Unix(int64(binary.BigEndian.Uint32(nonce)), 0)
	if time.Now().After(exp) {
		return false
	}

	gcm, _ := cipher.NewGCM(common.Cfg.Blk)
	p, _ := gcm.Open(nil, nonce, idbuf, nil)
	return string(p) == id
}

func MakeUserToken(u *model.User) string {
	if u == nil {
		return ""
	}

	length := len(u.ID) + 1 + len(u.Session)
	length = (length + 7) / 8 * 8

	x := make([]byte, length)
	copy(x, u.Session)
	copy(x[len(u.Session)+1:], u.ID)

	for i := 0; i <= len(x)-16; i += 8 {
		common.Cfg.Blk.Encrypt(x[i:], x[i:])
	}
	return base64.StdEncoding.EncodeToString(x)
}
