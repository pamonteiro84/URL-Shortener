package auth

import "testing"

func TestHashPassword_VerifyPassword_RoundTrip(t *testing.T) {
      hash, err := HashPassword("hunter2")
      if err != nil {
              t.Fatalf("HashPassword() unexpected error: %v", err)
      }

      ok, err := VerifyPassword(hash, "hunter2")
      if err != nil {
              t.Fatalf("VerifyPassword() unexpected error: %v", err)
      }
      if !ok {
              t.Error("VerifyPassword() = false, want true for the correct password")
      }
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
      hash, err := HashPassword("hunter2")
      if err != nil {
              t.Fatalf("HashPassword() unexpected error: %v", err)
      }

      ok, err := VerifyPassword(hash, "wrongpassword")
      if err != nil {
              t.Fatalf("VerifyPassword() unexpected error: %v", err)
      }
      if ok {
              t.Error("VerifyPassword() = true, want false for the wrong password")
      }
}

func TestHashPassword_DifferentSaltEachTime(t *testing.T) {
      first, err := HashPassword("hunter2")
      if err != nil {
              t.Fatalf("HashPassword() unexpected error: %v", err)
      }
      second, err := HashPassword("hunter2")
      if err != nil {
              t.Fatalf("HashPassword() unexpected error: %v", err)
      }

      if first == second {
              t.Error("HashPassword() produced the same hash twice for the same password — salt isn't random")
      }
}

func TestVerifyPassword_InvalidFormat(t *testing.T) {
      _, err := VerifyPassword("not-a-valid-hash", "anything")
      if err == nil {
              t.Error("VerifyPassword() expected error for malformed hash, got nil")
      }
}