package cryptoExt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"time"

	"github.com/dgrijalva/jwt-go"
)

func ProduceJwtToken(secretKey []byte, uid int64) string {

	return ProduceJwtTokenByBody(secretKey, jwt.MapClaims{
		"sub": strconv.FormatInt(uid, 10),
	})
}

func ProduceJwtTokenTime(secretKey []byte, body jwt.MapClaims, second time.Duration) string {
	body["exp"] = time.Now().Add(second).Unix()
	body["iat"] = time.Now().Unix()
	return ProduceToken(secretKey, body)
}

func ProduceJwtTokenByBody(secretKey []byte, body jwt.MapClaims) string {
	return ProduceJwtTokenTime(secretKey, body, time.Hour*time.Duration(5))
}

func ProduceRefreshToken(secretKey []byte, uid int64) string {
	return ProduceRefreshTokenByBody(secretKey, jwt.MapClaims{
		"sub": strconv.FormatInt(uid, 10),
	})
}

func ProduceRefreshTokenByBody(secretKey []byte, body jwt.MapClaims) string {
	return ProduceJwtTokenTime(secretKey, body, time.Hour*time.Duration(24*7))
}

func ProduceRefreshTokenBodyTime(secretKey []byte, body jwt.MapClaims, time time.Duration) string {
	return ProduceJwtTokenTime(secretKey, body, time)
}

func ProduceToken(secretKey []byte, claims jwt.MapClaims) string {
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims

	// Sign and get the complete encoded token as a string using the secret
	tokenString, _ := token.SignedString(secretKey)

	return tokenString
}

func ParseUserId(secretKey []byte, tokenString string) int64 {
	isOk, claims := ParseTokenBody(secretKey, tokenString)
	if isOk {
		i, ok := claims["sub"]
		if !ok {
			return 0
		}

		var usrId int64
		userIdStr, ok := i.(string)

		if ok {
			usrId, _ = strconv.ParseInt(userIdStr, 10, 64)
		} else {
			userIdN, _ := i.(json.Number)
			usrId, _ = (userIdN).Int64()
		}

		return usrId
	} else {
		return 0
	}
}

func ParseTokenBodyValue(claims jwt.MapClaims, key string, kind reflect.Kind) int64 {
	switch kind {
	case reflect.Bool:
		value, ok := claims[key].(bool)
		if ok && value {
			return 1
		} else {
			return 0
		}

	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:

		value, ok := claims[key].(json.Number)
		if ok {
			valueInt64, _ := value.Int64()
			return valueInt64
		} else {
			return 0
		}
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		value, ok := claims[key].(json.Number)
		if ok {
			valueInt64, _ := value.Int64()
			return int64(valueInt64)
		} else {
			return 0
		}
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		value, ok := claims[key].(json.Number)
		if ok {
			valuefloat64, _ := value.Float64()
			return int64(valuefloat64)
		} else {
			return 0
		}
	}
	return 0
}

func ParseTokenBody(secretKey []byte, tokenString string) (bool, jwt.MapClaims) {
	if len(tokenString) == 0 {
		return false, nil
	}

	parser := new(jwt.Parser)
	parser.UseJSONNumber = true
	parseToken, parseErr := parser.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if parseErr != nil {
		return false, nil
	}
	return parseToken.Valid, parseToken.Claims.(jwt.MapClaims)
}

func ParseTokenBodyNotValidation(secretKey []byte, tokenString string) (bool, jwt.MapClaims) {
	if len(tokenString) == 0 {
		return false, nil
	}

	parser := new(jwt.Parser)
	parser.UseJSONNumber = true
	parser.SkipClaimsValidation = true
	parseToken, parseErr := parser.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if parseErr != nil {
		return false, nil
	}
	return parseToken.Valid, parseToken.Claims.(jwt.MapClaims)
}
