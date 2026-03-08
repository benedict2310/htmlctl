package links

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

func UnsubscribeURL(publicBaseURL, secret string, subscriberID int64) string {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	return fmt.Sprintf("%s/newsletter/unsubscribe?token=%s", base, GenerateUnsubscribeToken(secret, subscriberID))
}

func GenerateUnsubscribeToken(secret string, subscriberID int64) string {
	payload := strconv.FormatInt(subscriberID, 10)
	mac := hmacFor(secret, payload)
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac)
}

func ParseUnsubscribeToken(secret, token string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid token")
	}
	subscriberID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || subscriberID <= 0 {
		return 0, fmt.Errorf("invalid token")
	}
	gotMAC, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid token")
	}
	wantMAC := hmacFor(secret, parts[0])
	if subtle.ConstantTimeCompare(gotMAC, wantMAC) != 1 {
		return 0, fmt.Errorf("invalid token")
	}
	return subscriberID, nil
}

func hmacFor(secret, payload string) []byte {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
