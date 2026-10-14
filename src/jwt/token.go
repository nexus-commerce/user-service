package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ttl = 24 * 7 * time.Hour
)

var ErrInvalid = errors.New("invalid token")

type claims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret []byte
	issuer string
}

type Options struct {
	Secret string
	Issuer string
}

func NewTokens(opts Options) *Tokens {
	return &Tokens{
		secret: []byte(opts.Secret),
		issuer: opts.Issuer,
	}
}

func (t *Tokens) Issue(userID string) (string, error) {
	c := claims{
		UserID:    userID,
		Issuer:    t.issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return token, nil
}

func (t *Tokens) Parse(token string) (string, error) {
	var c claims
	keyFunc := func(*jwt.Token) (any, error) { return t.secret, nil }
	_, err := jwt.ParseWithClaims(token, &c, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if c.UserID == "" {
		return "", fmt.Errorf("%w: missing user ID", ErrInvalid)
	}

	return c.UserID, nil
}
