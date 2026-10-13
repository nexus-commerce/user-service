package token

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

type JWT struct {
	secret []byte
	issuer string
}

type Options struct {
	Secret string
	Issuer string
}

func New(opts Options) *JWT {
	return &JWT{
		secret: []byte(opts.Secret),
		issuer: opts.Issuer,
	}
}

func (j *JWT) Issue(userID string) (string, error) {
	c := claims{
		UserID:    userID,
		Issuer:    j.issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(j.secret)
}

func (j *JWT) Parse(token string) (string, error) {
	var c claims
	keyFunc := func(*jwt.Token) (any, error) { return j.secret, nil }
	_, err := jwt.ParseWithClaims(token, &c, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(j.issuer),
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
