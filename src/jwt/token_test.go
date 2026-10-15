package jwt

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueParseRoundTrip(t *testing.T) {
	tokens := NewTokens(Options{Secret: "secret", Issuer: "user-service"})
	id := uuid.New()

	token, err := tokens.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tokens.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("Parse = %v, want %v", got, id)
	}
}

func TestParseRejectsBadUserID(t *testing.T) {
	tokens := NewTokens(Options{Secret: "secret", Issuer: "user-service"})
	exp := time.Now().Add(time.Hour).Unix()

	tests := []struct {
		name   string
		claims jwt.MapClaims
	}{
		{name: "malformed", claims: jwt.MapClaims{"userId": "x", "iss": "user-service", "exp": exp}},
		{name: "empty", claims: jwt.MapClaims{"userId": "", "iss": "user-service", "exp": exp}},
		{name: "missing", claims: jwt.MapClaims{"iss": "user-service", "exp": exp}},
		{name: "nil uuid", claims: jwt.MapClaims{"userId": uuid.Nil().String(), "iss": "user-service", "exp": exp}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tt.claims).SignedString([]byte("secret"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Parse(token); !errors.Is(err, ErrInvalid) {
				t.Errorf("Parse error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}
