package cryptoExt

import (
	"golang.org/x/crypto/bcrypt"
)

func ProduceBcrypt(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func CheckBcrypt(bcryptHash, passwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(bcryptHash), []byte(passwd)) == nil
}
