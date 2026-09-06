package auth

import (
      "crypto/rand"
      "encoding/base64"
      "fmt"
	  "strings"
      "golang.org/x/crypto/argon2"
	  "errors"
	  "crypto/subtle"
)

const (
      argonTime    = 2
      argonMemory  = 19 * 1024 // KiB (~19 MiB) — recomendação OWASP
      argonThreads = 1
      argonKeyLen  = 32
      saltLen      = 16
)

func HashPassword(password string) (string, error) {
      salt := make([]byte, saltLen)
      if _, err := rand.Read(salt); err != nil {
              return "", err
      }

      hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

      encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
      encodedHash := base64.RawStdEncoding.EncodeToString(hash)

      return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
              argon2.Version, argonMemory, argonTime, argonThreads, encodedSalt, encodedHash), nil
}

func VerifyPassword(encodedHash, password string) (bool, error) {
      parts := strings.Split(encodedHash, "$")
      if len(parts) != 6 {
              return false, errors.New("invalid hash format")
      }

      var memory, time uint32
      var threads uint8
      if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
              return false, err
      }

      salt, err := base64.RawStdEncoding.DecodeString(parts[4])
      if err != nil {
              return false, err
      }
      storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
      if err != nil {
              return false, err
      }

      computedHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(storedHash)))

      return subtle.ConstantTimeCompare(storedHash, computedHash) == 1, nil
}