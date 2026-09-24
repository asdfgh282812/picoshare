package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mtlynch/picoshare/i18n"
	"github.com/mtlynch/picoshare/picoshare"
)

var contextKeyLocalizer = new(contextKey{name: "localizer"})

// languageCookieName holds a visitor's chosen interface language. PicoShare
// sets it for both anonymous and logged-in visitors so a choice survives
// logging out; logged-in visitors also get it saved to their account so it
// follows them to another browser.
const languageCookieName = "ps_lang"

const languageCookieLifetime = 365 * 24 * time.Hour

// resolveLanguage attaches an i18n.Localizer to the request context,
// resolved in order: the logged-in user's saved preference, the ps_lang
// cookie, the administrator's configured site default (or the visitor's
// Accept-Language header when the site default is "auto"), and finally
// English. It must run after loadSession so a logged-in user's preference is
// available.
func (s Server) resolveLanguage(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := s.resolveLanguageForRequest(r)
		ctx := context.WithValue(r.Context(), contextKeyLocalizer, i18n.New(lang.String()))
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s Server) resolveLanguageForRequest(r *http.Request) picoshare.Language {
	if user, ok := currentUser(r.Context()); ok && !user.PreferredLanguage.Empty() {
		return user.PreferredLanguage
	}

	if cookie, err := r.Cookie(languageCookieName); err == nil {
		if lang, err := picoshare.NewLanguage(cookie.Value); err == nil {
			return lang
		}
	}

	settings, err := s.store.ReadSettings()
	if err != nil {
		log.Printf("failed to read settings while resolving language: %v", err)
		return picoshare.LanguageEnglish
	}
	if !settings.DefaultLanguage.IsAuto() {
		return settings.DefaultLanguage.Language()
	}

	return languageFromAcceptLanguage(r.Header.Get("Accept-Language"))
}

// languageFromAcceptLanguage matches an Accept-Language header against
// PicoShare's supported languages, preferring the visitor's highest-quality
// choice. It treats any Chinese variant (zh-TW, zh-CN, zh-Hans, ...) as a
// match for Traditional Chinese, since it's the only Chinese interface
// PicoShare offers. It falls back to English.
func languageFromAcceptLanguage(header string) picoshare.Language {
	type candidate struct {
		tag string
		q   float64
	}
	var candidates []candidate
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tag := part
		q := 1.0
		if i := strings.Index(part, ";"); i != -1 {
			tag = strings.TrimSpace(part[:i])
			if params := strings.TrimSpace(part[i+1:]); strings.HasPrefix(params, "q=") {
				if v, err := strconv.ParseFloat(strings.TrimPrefix(params, "q="), 64); err == nil {
					q = v
				}
			}
		}
		if tag != "" {
			candidates = append(candidates, candidate{tag: tag, q: q})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].q > candidates[j].q
	})
	for _, c := range candidates {
		tag := strings.ToLower(c.tag)
		if strings.HasPrefix(tag, "zh") {
			return picoshare.LanguageTraditionalChinese
		}
		if strings.HasPrefix(tag, "en") {
			return picoshare.LanguageEnglish
		}
	}
	return picoshare.LanguageEnglish
}

func localizerFromContext(ctx context.Context) i18n.Localizer {
	if l, ok := ctx.Value(contextKeyLocalizer).(i18n.Localizer); ok {
		return l
	}
	return i18n.New(i18n.English)
}

// languagePut lets a visitor choose PicoShare's interface language. It always
// sets the ps_lang cookie, and additionally saves the choice to the account
// of a logged-in visitor.
func (s Server) languagePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		lang, err := picoshare.NewLanguage(payload.Language)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid language: %v", err), http.StatusBadRequest)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     languageCookieName,
			Value:    lang.String(),
			Path:     "/",
			Expires:  s.now().Add(languageCookieLifetime),
			HttpOnly: true,
			Secure:   requestIsHTTPS(r),
			SameSite: http.SameSiteLaxMode,
		})

		if user, ok := currentUser(r.Context()); ok {
			if err := s.store.UpdateUserLanguage(user.ID, lang); err != nil {
				log.Printf("failed to save language preference for user %v: %v", user.ID, err)
				http.Error(w, "Failed to save language preference", http.StatusInternalServerError)
				return
			}
		}
	}
}
