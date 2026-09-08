package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"crypto/aes"
	"io"
	"crypto/rand"
	"crypto/cipher"
	"encoding/hex"
	"crypto/hmac"
	"crypto/sha256"
	"os"
	

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func AesPy(email string) (string, error) {
	cmd := exec.Command("python", "utils/aes.py", email)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("python error: %s", stderr.String())
	}

	encryptedResult := strings.TrimSpace(out.String())
	return encryptedResult, nil
}

func AesGo(email string) (string, error) {
	key := []byte(os.Getenv("keyAesGo"))

	block,err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _,err := io.ReadFull(rand.Reader, nonce); err != nil{
		return "", err
	}

	encrypted := gcm.Seal(nil, nonce, []byte(email), nil)
	return hex.EncodeToString(encrypted), nil
}

func GenerateBlindIndex(email string, hmacSecret []byte) string{
	h := hmac.New(sha256.New, hmacSecret)
	h.Write([]byte(email))
	return hex.EncodeToString(h.Sum(nil))
}