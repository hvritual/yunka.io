package cryptoExt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseTokenBodyWithValidation(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	claims := jwt.MapClaims{
		"sub": "user-1", "iss": "yunka", "aud": "gateway",
		"iat": time.Now().Add(-time.Minute).Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := ProduceToken(secret, claims)
	if ok, _ := ParseTokenBodyWithValidation(secret, token, "yunka", "gateway"); !ok {
		t.Fatal("valid token was rejected")
	}
	if ok, _ := ParseTokenBodyWithValidation(secret, token, "other", "gateway"); ok {
		t.Fatal("wrong issuer was accepted")
	}
	if ok, _ := ParseTokenBodyWithValidation(secret, token, "yunka", "other"); ok {
		t.Fatal("wrong audience was accepted")
	}
}

func TestExpiredAndUnsignedTokensAreRejected(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	expired := ProduceToken(secret, jwt.MapClaims{
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	if ok, _ := ParseTokenBody(secret, expired); ok {
		t.Fatal("expired token was accepted")
	}
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	})
	token, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := ParseTokenBody(secret, token); ok {
		t.Fatal("unsigned token was accepted")
	}
}
