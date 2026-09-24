package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/mtlynch/picoshare/handlers/parse"
	"github.com/mtlynch/picoshare/picoshare"
)

func (s Server) settingsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := settingsFromRequest(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		if err := s.store.UpdateSettings(settings); err != nil {
			log.Printf("failed to save settings: %v", err)
			http.Error(w, fmt.Sprintf("Failed to save settings: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func settingsFromRequest(r *http.Request) (picoshare.Settings, error) {
	var payload struct {
		DefaultExpirationDays        uint16 `json:"defaultExpirationDays"`
		DefaultNeverExpire           bool   `json:"defaultNeverExpire"`
		DownloadHistoryRetentionDays uint16 `json:"downloadHistoryRetentionDays"`
		KeepDownloadHistoryForever   bool   `json:"keepDownloadHistoryForever"`
		DefaultLanguage              string `json:"defaultLanguage"`
	}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		log.Printf("failed to decode JSON request: %v", err)
		return picoshare.Settings{}, err
	}

	defaultNeverExpire := payload.DefaultNeverExpire

	var defaultLifetime picoshare.FileLifetime
	if defaultNeverExpire {
		defaultLifetime = picoshare.FileLifetimeInfinite
	} else if defaultLifetime, err = parse.FileLifetime(payload.DefaultExpirationDays); err != nil {
		return picoshare.Settings{}, err
	}

	retention := picoshare.KeepDownloadHistoryForever
	if !payload.KeepDownloadHistoryForever {
		if retention, err = picoshare.NewDownloadHistoryRetentionInDays(payload.DownloadHistoryRetentionDays); err != nil {
			return picoshare.Settings{}, err
		}
	}

	defaultLanguage, err := picoshare.NewSiteDefaultLanguage(payload.DefaultLanguage)
	if err != nil {
		return picoshare.Settings{}, err
	}

	return picoshare.Settings{
		DefaultFileLifetime:      defaultLifetime,
		DownloadHistoryRetention: retention,
		DefaultLanguage:          defaultLanguage,
	}, nil
}
