package auth

import (
      "testing"
      "time"
)

func TestGenerateAccessToken_ValidateAccessToken_RoundTrip(t *testing.T) {
      secret := []byte("test-secret")

      token, err := GenerateAccessToken(42, secret, time.Minute)
      if err != nil {
              t.Fatalf("GenerateAccessToken() unexpected error: %v", err)
      }

      userID, err := ValidateAccessToken(token, secret)
      if err != nil {
              t.Fatalf("ValidateAccessToken() unexpected error: %v", err)
      }
      if userID != 42 {
              t.Errorf("ValidateAccessToken() userID = %d, want 42", userID)
      }
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
      token, err := GenerateAccessToken(42, []byte("correct-secret"), time.Minute)
      if err != nil {
              t.Fatalf("GenerateAccessToken() unexpected error: %v", err)
      }

      _, err = ValidateAccessToken(token, []byte("wrong-secret"))
      if err == nil {
              t.Error("ValidateAccessToken() expected error for wrong secret, got nil")
      }
}

func TestValidateAccessToken_Expired(t *testing.T) {
      secret := []byte("test-secret")

      token, err := GenerateAccessToken(42, secret, -time.Minute)
      if err != nil {
              t.Fatalf("GenerateAccessToken() unexpected error: %v", err)
      }

      _, err = ValidateAccessToken(token, secret)
      if err == nil {
              t.Error("ValidateAccessToken() expected error for expired token, got nil")
      }
}