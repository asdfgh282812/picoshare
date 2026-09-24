package picoshare_test

import (
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestSessionTokenRoundTrip(t *testing.T) {
	token := picoshare.NewSessionToken()

	parsed, err := picoshare.SessionTokenFromString(token.String())
	if err != nil {
		t.Fatalf("failed to parse a freshly generated token: %v", err)
	}
	if got, want := parsed.String(), token.String(); got != want {
		t.Errorf("String()=%s, want %s", got, want)
	}
}

func TestSessionTokenFromStringRejectsMalformedInput(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "empty text is invalid",
			input:           "",
			isValidExpected: false,
		},
		{
			explanation:     "text that isn't base64 is invalid",
			input:           "not valid base64!!",
			isValidExpected: false,
		},
		{
			explanation:     "a token that's too short is invalid",
			input:           "YWJj",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.SessionTokenFromString(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestSessionTokenHashIsDeterministicAndUnique(t *testing.T) {
	a := picoshare.NewSessionToken()
	b := picoshare.NewSessionToken()

	aAgain, err := picoshare.SessionTokenFromString(a.String())
	if err != nil {
		t.Fatalf("failed to re-parse token: %v", err)
	}

	if got, want := aAgain.Hash().String(), a.Hash().String(); got != want {
		t.Errorf("hashing the same token twice produced different results: got %s, want %s", got, want)
	}
	if a.Hash().String() == b.Hash().String() {
		t.Error("two different tokens produced the same hash")
	}
}
