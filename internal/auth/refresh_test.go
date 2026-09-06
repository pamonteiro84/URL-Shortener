package auth

import "testing"

func TestGenerateRefreshToken_NoError(t *testing.T) {
      token, err := GenerateRefreshToken()
      if err != nil {
              t.Fatalf("GenerateRefreshToken() unexpected error: %v", err)
      }
      if token == "" {
              t.Error("GenerateRefreshToken() returned empty token")
      }
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
      first, err := GenerateRefreshToken()
      if err != nil {
              t.Fatalf("GenerateRefreshToken() unexpected error: %v", err)
      }
      second, err := GenerateRefreshToken()
      if err != nil {
              t.Fatalf("GenerateRefreshToken() unexpected error: %v", err)
      }

      if first == second {
              t.Errorf("GenerateRefreshToken() returned the same token twice: %q", first)
      }
}