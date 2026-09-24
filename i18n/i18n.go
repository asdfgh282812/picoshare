// Package i18n translates PicoShare's user interface. It embeds a flat
// key-value catalog for each supported language and resolves lookups against
// it at render time.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
)

//go:embed locales/*.json
var localesFS embed.FS

// English is the fallback language: every key must exist in its catalog, and
// every other catalog falls back to it for a missing key.
const English = "en"

// TraditionalChinese is PicoShare's Traditional Chinese interface.
const TraditionalChinese = "zh-TW"

// SupportedLanguages lists every language code PicoShare can render its
// interface in, in the order they should appear in a language picker.
var SupportedLanguages = []string{English, TraditionalChinese}

var catalogs = loadCatalogs()

func loadCatalogs() map[string]map[string]string {
	catalogs := make(map[string]map[string]string, len(SupportedLanguages))
	for _, lang := range SupportedLanguages {
		data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.json", lang))
		if err != nil {
			panic(fmt.Sprintf("i18n: failed to read locale %q: %v", lang, err))
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			panic(fmt.Sprintf("i18n: failed to parse locale %q: %v", lang, err))
		}
		catalogs[lang] = messages
	}
	return catalogs
}

// IsSupported reports whether lang is one of SupportedLanguages.
func IsSupported(lang string) bool {
	_, ok := catalogs[lang]
	return ok
}

// Localizer translates message keys into a single language, falling back to
// English for any key the target language's catalog doesn't define.
type Localizer struct {
	lang string
}

// New returns a Localizer for lang. It falls back to English if lang isn't
// supported.
func New(lang string) Localizer {
	if !IsSupported(lang) {
		lang = English
	}
	return Localizer{lang: lang}
}

// Lang returns the resolved language code, suitable for an HTML lang
// attribute or a persisted preference.
func (l Localizer) Lang() string {
	return l.lang
}

// T looks up key in the active language and, if given, formats it with args
// via fmt.Sprintf. It falls back to the English message, and finally to the
// key itself, if no catalog defines it.
func (l Localizer) T(key string, args ...any) string {
	msg, ok := catalogs[l.lang][key]
	if !ok {
		msg, ok = catalogs[English][key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// THTML behaves like T, but returns the result as trusted HTML instead of
// escaping it. Only use it for messages that PicoShare's own locale files
// define, such as marketing copy with an inline link, never for a message
// built from user-supplied content.
func (l Localizer) THTML(key string, args ...any) template.HTML {
	// #nosec -- the message comes from PicoShare's own embedded locale files,
	// not from user input.
	return template.HTML(l.T(key, args...))
}

// Messages returns the given keys resolved in the active language, for
// embedding a page's client-side translations as JSON.
func (l Localizer) Messages(keys ...string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = l.T(k)
	}
	return out
}

// Keys returns every key defined for lang, sorted. It exists for tests that
// verify every supported language defines the same set of keys.
func Keys(lang string) []string {
	keys := make([]string, 0, len(catalogs[lang]))
	for k := range catalogs[lang] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
