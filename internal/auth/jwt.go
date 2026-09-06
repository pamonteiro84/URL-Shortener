package auth

import (
      "time"
      "github.com/golang-jwt/jwt/v5"
)

type claims struct {
      UserID uint `json:"user_id"`
      jwt.RegisteredClaims
}

func GenerateAccessToken(userID uint, secret []byte, ttl time.Duration) (string, error) {
      c := claims{
              UserID: userID,
              RegisteredClaims: jwt.RegisteredClaims{
                      ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
                      IssuedAt:  jwt.NewNumericDate(time.Now()),
              },
      }

      token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
      return token.SignedString(secret)
}

func ValidateAccessToken(tokenString string, secret []byte) (uint, error) {
      c := &claims{}

      _, err := jwt.ParseWithClaims(tokenString, c, func(t *jwt.Token) (interface{}, error) {
              return secret, nil
      })
      if err != nil {
              return 0, err
      }

      return c.UserID, nil
}