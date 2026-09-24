package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

// resolvedLang extracts PicoShare's resolved interface language from a
// rendered page's <html lang="..."> attribute.
func resolvedLang(t testing.TB, body string) string {
	t.Helper()
	const marker = `<html lang="`
	i := strings.Index(body, marker)
	if i == -1 {
		t.Fatalf("response body has no <html lang=\"...\"> attribute: %s", body)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j == -1 {
		t.Fatalf("malformed <html lang=\"...\"> attribute: %s", body)
	}
	return rest[:j]
}

// TestLanguageResolutionOrder covers every layer of PicoShare's language
// resolution: a logged-in user's saved preference, the ps_lang cookie, the
// administrator's site default, and finally the visitor's Accept-Language
// header.
func TestLanguageResolutionOrder(t *testing.T) {
	for _, tt := range []struct {
		explanation    string
		acceptLanguage string
		langCookie     string
		siteDefault    string
		userPreference string
		wantLang       string
	}{
		{
			explanation: "falls back to English when nothing else applies",
			wantLang:    "en",
		},
		{
			explanation:    "matches a Traditional Chinese Accept-Language header",
			acceptLanguage: "zh-TW",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "treats zh-CN as a match for Traditional Chinese",
			acceptLanguage: "zh-CN,zh;q=0.9,en;q=0.5",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "honors Accept-Language quality values out of order",
			acceptLanguage: "en;q=0.5,zh-TW;q=0.9",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "falls back to English for an unsupported Accept-Language",
			acceptLanguage: "fr-FR,fr;q=0.9",
			wantLang:       "en",
		},
		{
			explanation:    "a valid ps_lang cookie overrides Accept-Language",
			acceptLanguage: "en-US",
			langCookie:     "zh-TW",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "an invalid ps_lang cookie value is ignored",
			acceptLanguage: "en-US",
			langCookie:     "klingon",
			wantLang:       "en",
		},
		{
			explanation:    "the site default overrides Accept-Language",
			acceptLanguage: "en-US",
			siteDefault:    "zh-TW",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "a language cookie overrides the site default",
			acceptLanguage: "en-US",
			siteDefault:    "zh-TW",
			langCookie:     "en",
			wantLang:       "en",
		},
		{
			explanation:    "auto site default falls through to Accept-Language",
			acceptLanguage: "zh-TW",
			siteDefault:    "auto",
			wantLang:       "zh-TW",
		},
		{
			explanation:    "a logged-in user's preference overrides the cookie",
			acceptLanguage: "en-US",
			langCookie:     "en",
			userPreference: "zh-TW",
			wantLang:       "zh-TW",
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)

			var loginCookie *http.Cookie
			if tt.userPreference != "" {
				user, cookie := mustLoginAsUser(t, &dataStore, "alice", mustParseTime("2023-01-01T00:00:00Z"))
				loginCookie = cookie
				lang, err := picoshare.NewLanguage(tt.userPreference)
				if err != nil {
					t.Fatalf("failed to construct language: %v", err)
				}
				if err := dataStore.UpdateUserLanguage(user.ID, lang); err != nil {
					t.Fatalf("failed to save user language preference: %v", err)
				}
			}

			if tt.siteDefault != "" {
				settings, err := dataStore.ReadSettings()
				if err != nil {
					t.Fatalf("failed to read settings: %v", err)
				}
				defaultLang, err := picoshare.NewSiteDefaultLanguage(tt.siteDefault)
				if err != nil {
					t.Fatalf("failed to construct site default language: %v", err)
				}
				settings.DefaultLanguage = defaultLang
				if err := dataStore.UpdateSettings(settings); err != nil {
					t.Fatalf("failed to save settings: %v", err)
				}
			}

			s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			if tt.langCookie != "" {
				req.AddCookie(&http.Cookie{Name: "ps_lang", Value: tt.langCookie})
			}
			if loginCookie != nil {
				req.AddCookie(loginCookie)
			}

			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)
			res := rec.Result()

			if got, want := res.StatusCode, http.StatusOK; got != want {
				t.Fatalf("GET / returned status %d, want %d", got, want)
			}

			if got, want := resolvedLang(t, rec.Body.String()), tt.wantLang; got != want {
				t.Errorf("resolved language=%q, want=%q", got, want)
			}
		})
	}
}

func TestLanguagePut(t *testing.T) {
	t.Run("sets the language cookie for an anonymous visitor", func(t *testing.T) {
		dataStore := test_sqlite.New(t)
		s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

		req := httptest.NewRequest(http.MethodPut, "/api/language", strings.NewReader(`{"language":"zh-TW"}`))
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		res := rec.Result()

		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Fatalf("PUT /api/language returned status %d, want %d", got, want)
		}

		var cookie *http.Cookie
		for _, c := range res.Cookies() {
			if c.Name == "ps_lang" {
				cookie = c
			}
		}
		if cookie == nil {
			t.Fatal("response did not set the ps_lang cookie")
		}
		if got, want := cookie.Value, "zh-TW"; got != want {
			t.Errorf("ps_lang cookie value=%q, want=%q", got, want)
		}
	})

	t.Run("saves the preference to a logged-in user's account", func(t *testing.T) {
		dataStore := test_sqlite.New(t)
		user, loginCookie := mustLoginAsUser(t, &dataStore, "alice", mustParseTime("2023-01-01T00:00:00Z"))
		s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

		req := httptest.NewRequest(http.MethodPut, "/api/language", strings.NewReader(`{"language":"zh-TW"}`))
		req.AddCookie(loginCookie)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		res := rec.Result()

		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Fatalf("PUT /api/language returned status %d, want %d", got, want)
		}

		users, err := dataStore.GetUsers()
		if err != nil {
			t.Fatalf("failed to read back users: %v", err)
		}
		var got picoshare.Language
		for _, u := range users {
			if u.ID == user.ID {
				got = u.PreferredLanguage
			}
		}
		if want := picoshare.LanguageTraditionalChinese; got != want {
			t.Errorf("saved preferred language=%v, want=%v", got, want)
		}
	})

	t.Run("rejects an unsupported language", func(t *testing.T) {
		dataStore := test_sqlite.New(t)
		s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

		req := httptest.NewRequest(http.MethodPut, "/api/language", strings.NewReader(`{"language":"klingon"}`))
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		res := rec.Result()

		if got, want := res.StatusCode, http.StatusBadRequest; got != want {
			t.Fatalf("PUT /api/language returned status %d, want %d", got, want)
		}
	})
}
