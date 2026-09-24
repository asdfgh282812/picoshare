package picoshare_test

import (
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestNewLanguage(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		raw         string
		wantErr     bool
	}{
		{
			explanation: "accepts English",
			raw:         "en",
		},
		{
			explanation: "accepts Traditional Chinese",
			raw:         "zh-TW",
		},
		{
			explanation: "rejects an unsupported language",
			raw:         "fr",
			wantErr:     true,
		},
		{
			explanation: "rejects an empty string",
			raw:         "",
			wantErr:     true,
		},
		{
			explanation: "rejects a case mismatch",
			raw:         "EN",
			wantErr:     true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			lang, err := picoshare.NewLanguage(tt.raw)
			if got, want := (err != nil), tt.wantErr; got != want {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got, want := lang.String(), tt.raw; got != want {
				t.Errorf("String()=%v, want=%v", got, want)
			}
			if lang.Empty() {
				t.Error("Empty()=true for a valid language")
			}
		})
	}
}

func TestLanguageZeroValueIsEmpty(t *testing.T) {
	var lang picoshare.Language
	if !lang.Empty() {
		t.Error("Empty()=false for the zero value")
	}
}

func TestNewSiteDefaultLanguage(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		raw         string
		wantIsAuto  bool
		wantErr     bool
	}{
		{
			explanation: "empty string means auto",
			raw:         "",
			wantIsAuto:  true,
		},
		{
			explanation: `"auto" means auto`,
			raw:         "auto",
			wantIsAuto:  true,
		},
		{
			explanation: "accepts a supported language",
			raw:         "zh-TW",
			wantIsAuto:  false,
		},
		{
			explanation: "rejects an unsupported language",
			raw:         "fr",
			wantErr:     true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			l, err := picoshare.NewSiteDefaultLanguage(tt.raw)
			if got, want := (err != nil), tt.wantErr; got != want {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got, want := l.IsAuto(), tt.wantIsAuto; got != want {
				t.Errorf("IsAuto()=%v, want=%v", got, want)
			}
			if !tt.wantIsAuto {
				if got, want := l.Language().String(), tt.raw; got != want {
					t.Errorf("Language().String()=%v, want=%v", got, want)
				}
			}
		})
	}
}

func TestSiteDefaultLanguageAutoString(t *testing.T) {
	if got, want := picoshare.SiteDefaultLanguageAuto.String(), "auto"; got != want {
		t.Errorf("String()=%v, want=%v", got, want)
	}
}
