package auth

import (
	"testing"
	"time"
)

var (
	email = "bubba@gump.com"
)

func TestGood(t *testing.T) {
	token := TokenFor(email, time.Now().Add(1*time.Hour))
	emailOut, err := IdFor(token)
	if err != nil {
		t.Errorf("Error getting id for token: %s", err)
	} else if email != emailOut {
		t.Errorf("Email '%s' did not match expected '%s'", emailOut, email)
	}
}

func TestBad(t *testing.T) {
	token := TokenFor(email, time.Now().Add(1*time.Hour))
	_, err := IdFor("big" + token)
	if err == nil {
		t.Errorf("Should have received error for bad token")
	}
}

func TestExpired(t *testing.T) {
	token := TokenFor(email, time.Now().Add(-1*time.Hour))
	_, err := IdFor(token)
	if err == nil {
		t.Errorf("Should have received error for bad token")
	}
}
