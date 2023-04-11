package cryptoExt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
)

/**
* @Description: TODO
* @date 2019-02-21
* @version V1.0
 */

func AesEncryptObj(keyBys []byte, payload interface{}) (string, error) {
	bys, err := json.Marshal(payload)
	if err != nil {
		return ``, err
	}

	payloadBys, err := AesEncrypt(keyBys, bys)
	if err != nil {
		return ``, err
	}

	return base64.RawStdEncoding.EncodeToString(payloadBys), nil
}

func AesDecryptObj(keyBys []byte, payloadData string, payload interface{}) error {
	bys, err := base64.RawStdEncoding.DecodeString(payloadData)
	if err != nil {
		return err
	}
	payloadBys, err := AesDecrypt(keyBys, bys)
	if err != nil {
		return err
	}

	err = json.Unmarshal(payloadBys, &payload)
	if err != nil {
		return err
	}

	return nil
}

func AesEncryptBase64String(keyBys []byte, bys []byte) (string, error) {
	payloadBys, err := AesEncrypt(keyBys, bys)
	if err != nil {
		return ``, err
	}

	return base64.RawStdEncoding.EncodeToString(payloadBys), nil
}

func AesDecryptBase64String(keyBys []byte, code string) ([]byte, error) {
	bys, err := base64.RawStdEncoding.DecodeString(code)
	if err != nil {
		return nil, err
	}
	return AesDecrypt(keyBys, bys)
}

func AesEncrypt(key, origData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AesDecrypt(key, crypted []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}



func ZeroPadding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{0}, padding)
	return append(ciphertext, padtext...)
}

func ZeroUnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}



func AesEncryptInfo(data interface{}, key []byte) (code string, err error) {
	bys, err := json.Marshal(data)

	if err != nil {
		return ``, err
	}

	payloadBys, err := AesEncrypt(key, bys)
	if err != nil {
		return ``, err
	}
	return base64.RawStdEncoding.EncodeToString(payloadBys), nil
}

func AesDecryptInfo(payload interface{}, code string, key []byte) error {
	bys, err := base64.RawStdEncoding.DecodeString(code)
	if err != nil {
		return err
	}

	payloadBys, err := AesDecrypt(key, bys)
	if err != nil {
		return err
	}
	err = json.Unmarshal(payloadBys, payload)
	if err != nil {
		return err
	}
	return nil
}
