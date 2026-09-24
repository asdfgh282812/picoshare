package i18n_test

import (
	"testing"

	"github.com/mtlynch/picoshare/i18n"
)

// TestLocaleKeysMatch verifies every supported language defines exactly the
// same set of keys as English, so a translator can never forget a key (it
// would silently fall back to English) or leave a stale one behind.
func TestLocaleKeysMatch(t *testing.T) {
	want := i18n.Keys(i18n.English)
	if len(want) == 0 {
		t.Fatal("English locale defines no keys")
	}

	for _, lang := range i18n.SupportedLanguages {
		if lang == i18n.English {
			continue
		}
		t.Run(lang, func(t *testing.T) {
			got := i18n.Keys(lang)

			wantSet := make(map[string]bool, len(want))
			for _, k := range want {
				wantSet[k] = true
			}
			gotSet := make(map[string]bool, len(got))
			for _, k := range got {
				gotSet[k] = true
			}

			for _, k := range want {
				if !gotSet[k] {
					t.Errorf("locale %q is missing key %q", lang, k)
				}
			}
			for _, k := range got {
				if !wantSet[k] {
					t.Errorf("locale %q defines unknown key %q (not in English)", lang, k)
				}
			}
		})
	}
}

func TestT(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		lang        string
		key         string
		args        []any
		want        string
	}{
		{
			explanation: "returns the message in the requested language",
			lang:        i18n.TraditionalChinese,
			key:         "nav.upload",
			want:        "上傳",
		},
		{
			explanation: "formats positional arguments",
			lang:        i18n.English,
			key:         "common.unlimited",
			want:        "Unlimited",
		},
		{
			explanation: "falls back to English for an unsupported language",
			lang:        "fr",
			key:         "nav.upload",
			want:        "Upload",
		},
		{
			explanation: "falls back to the key itself when no catalog defines it",
			lang:        i18n.English,
			key:         "does.not.exist",
			want:        "does.not.exist",
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			l := i18n.New(tt.lang)
			if got := l.T(tt.key, tt.args...); got != tt.want {
				t.Errorf("T(%q)=%q, want=%q", tt.key, got, tt.want)
			}
		})
	}
}

func TestNewFallsBackToEnglishForUnsupportedLanguage(t *testing.T) {
	l := i18n.New("klingon")
	if got, want := l.Lang(), i18n.English; got != want {
		t.Errorf("Lang()=%v, want=%v", got, want)
	}
}
