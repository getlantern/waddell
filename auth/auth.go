// Package auth provides primitives for authenticating waddell clients
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DELIM = "|"
)

var (
	KEY = []byte(os.Getenv("SECRET_AUTH_TOKEN_KEY"))
)

// TokenFor generates a token consisting of
// <id>|<expiration unix seconds>|<base 64 encoded hmac-sha256>
//
// See:
//    http://security.stackexchange.com/questions/30707/demystifying-web-authentication-stateless-session-cookies
//    http://security.stackexchange.com/questions/2823/is-it-possible-to-have-authentication-without-state
//    http://security.stackexchange.com/questions/24730/is-my-session-less-authentication-system-secure
//
func TokenFor(id string, expiration time.Time) string {
	encodedMac := base64.StdEncoding.EncodeToString(macFor(id, expiration.Unix()))
	return fmt.Sprintf("%s%s%d%s%s", id, DELIM, expiration.Unix(), DELIM, encodedMac)
}

// IdFor returns the id from the given token, if and only if the included hmac
// passes validation and the expiration date hasn't passed.
func IdFor(token string) (string, error) {
	parts := strings.Split(token, DELIM)
	if len(parts) != 3 {
		return "", fmt.Errorf("Wrong token format")
	}
	id, exp, encodedMac := parts[0], parts[1], parts[2]
	unixSeconds, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return "", fmt.Errorf("Unable to parse expiration %s: %s", exp, err)
	}
	expiration := time.Unix(unixSeconds, 0)
	if time.Now().After(expiration) {
		return "", fmt.Errorf("Token expired")
	}
	themac, err := base64.StdEncoding.DecodeString(encodedMac)
	if err != nil {
		return "", fmt.Errorf("Unable to decode mac '%s': %s", encodedMac, err)
	}
	macValid := hmac.Equal(macFor(id, unixSeconds), themac)
	if macValid {
		return id, nil
	} else {
		return "", fmt.Errorf("Invalid MAC")
	}
}

func macFor(id string, unixSeconds int64) []byte {
	data := fmt.Sprintf("%s%s%d", id, DELIM, unixSeconds)
	mac := hmac.New(sha256.New, KEY)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
