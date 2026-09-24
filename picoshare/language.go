package picoshare

import "fmt"

// Language identifies one of PicoShare's supported interface languages.
type Language struct {
	value string
}

var (
	// LanguageEnglish is PicoShare's default interface language.
	LanguageEnglish = Language{value: "en"}
	// LanguageTraditionalChinese is PicoShare's Traditional Chinese interface.
	LanguageTraditionalChinese = Language{value: "zh-TW"}
)

var supportedLanguages = []Language{LanguageEnglish, LanguageTraditionalChinese}

// NewLanguage constructs a Language from a user- or admin-supplied code, such
// as a form field or cookie value.
func NewLanguage(raw string) (Language, error) {
	for _, l := range supportedLanguages {
		if l.value == raw {
			return l, nil
		}
	}
	return Language{}, fmt.Errorf("unsupported language: %q", raw)
}

// Empty reports whether l is the zero value, meaning "no preference."
func (l Language) Empty() bool {
	return l.value == ""
}

func (l Language) String() string {
	return l.value
}

// SiteDefaultLanguage is the administrator-configured fallback language for
// visitors who haven't chosen one themselves. The zero value means "match
// the visitor's browser."
type SiteDefaultLanguage struct {
	value string
}

// SiteDefaultLanguageAuto tells PicoShare to match each visitor's browser
// language instead of a fixed site default.
var SiteDefaultLanguageAuto = SiteDefaultLanguage{}

// NewSiteDefaultLanguage constructs a SiteDefaultLanguage from an admin-
// supplied code. An empty string or "auto" both mean SiteDefaultLanguageAuto.
func NewSiteDefaultLanguage(raw string) (SiteDefaultLanguage, error) {
	if raw == "" || raw == "auto" {
		return SiteDefaultLanguageAuto, nil
	}
	if _, err := NewLanguage(raw); err != nil {
		return SiteDefaultLanguage{}, fmt.Errorf("unsupported site default language: %w", err)
	}
	return SiteDefaultLanguage{value: raw}, nil
}

// IsAuto reports whether visitors should get their browser's language rather
// than a fixed site default.
func (l SiteDefaultLanguage) IsAuto() bool {
	return l.value == ""
}

// Language returns the fixed site default language. Only meaningful when
// IsAuto reports false.
func (l SiteDefaultLanguage) Language() Language {
	if l.IsAuto() {
		return Language{}
	}
	lang, _ := NewLanguage(l.value)
	return lang
}

func (l SiteDefaultLanguage) String() string {
	if l.IsAuto() {
		return "auto"
	}
	return l.value
}
