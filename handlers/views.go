package handlers

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mileusna/useragent"

	"github.com/mtlynch/picoshare/build"
	"github.com/mtlynch/picoshare/handlers/parse"
	"github.com/mtlynch/picoshare/i18n"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

//go:embed templates
var templatesFS embed.FS

type commonProps struct {
	Title           string
	IsAuthenticated bool
	IsAdmin         bool
	Username        string
	CspNonce        string
	L               i18n.Localizer
	Lang            string
	SupportedLangs  []string
}

func (s Server) indexGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticated(r.Context()) {
			s.uploadGet()(w, r)
			return
		}
		renderTemplate(w, t, struct {
			commonProps
		}{
			commonProps: makeCommonProps("title.index", r.Context()),
		})
	}
}

func (s Server) guestLinkIndexGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format(time.DateOnly)
		},
		"formatSizeLimit": func(limit picoshare.GuestUploadMaxFileBytes, l i18n.Localizer) string {
			if limit == picoshare.GuestUploadUnlimitedFileSize {
				return l.T("common.unlimited")
			}
			b := uint64(*limit)
			const unit = 1024

			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "kMGTPE"[exp])
		},
		"formatCountLimit": func(limit picoshare.GuestUploadCountLimit, l i18n.Localizer) string {
			if limit == picoshare.GuestUploadUnlimitedFileUploads {
				return l.T("common.unlimited")
			}
			return fmt.Sprintf("%d", int(*limit))
		},
		"formatExpiration": func(et picoshare.ExpirationTime, l i18n.Localizer) string {
			if et == picoshare.NeverExpire {
				return l.T("lifetime.never")
			}
			t := time.Time(et)
			delta := t.Sub(s.now())
			days := math.Abs(delta.Hours()) / 24
			if delta.Seconds() < 0 {
				return l.T("expiration.withDaysAgo", t.Format(time.DateOnly), days)
			}
			return l.T("expiration.withDays", t.Format(time.DateOnly), days)
		},
		"friendlyLifetime":               friendlyLifetimeName,
		"guestLinkUploadProgressPercent": guestLinkUploadProgressPercent,
	}

	t := parseTemplatesWithFuncs(fns, "templates/pages/guest-link-index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := currentUser(r.Context())
		links, err := s.store.GetGuestLinks(user.ID)
		if err != nil {
			log.Printf("failed to retrieve guest links: %v", err)
			http.Error(w, "Failed to retrieve guest links", http.StatusInternalServerError)
			return
		}

		sort.Slice(links, func(i, j int) bool {
			return links[i].Created.After(links[j].Created)
		})

		renderTemplate(w, t, struct {
			commonProps
			GuestLinks []picoshare.GuestLink
		}{
			commonProps: makeCommonProps("title.guestLinks", r.Context()),
			GuestLinks:  links,
		})
	}
}

func (s Server) guestLinksNewGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatExpiration": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"formatLifetime": func(flt picoshare.FileLifetime) string {
			return flt.String()
		},
		"friendlyLifetime": friendlyLifetimeName,
	}

	t := parseTemplatesWithFuncs(fns, "templates/pages/guest-link-create.html")

	return func(w http.ResponseWriter, r *http.Request) {
		l := localizerFromContext(r.Context())
		type expirationOption struct {
			FriendlyName string
			Expiration   time.Time
			IsDefault    bool
		}
		type fileLifetimeOption struct {
			FileLifetime picoshare.FileLifetime
			IsDefault    bool
		}
		renderTemplate(w, t, struct {
			commonProps
			ExpirationOptions   []expirationOption
			FileLifetimeOptions []fileLifetimeOption
		}{
			commonProps: makeCommonProps("title.guestLinkNew", r.Context()),
			ExpirationOptions: []expirationOption{
				{friendlyLifetimeName(picoshare.NewFileLifetimeInDays(1), l), s.now().AddDate(0, 0, 1), false},
				{friendlyLifetimeName(picoshare.NewFileLifetimeInDays(7), l), s.now().AddDate(0, 0, 7), false},
				{friendlyLifetimeName(picoshare.NewFileLifetimeInDays(30), l), s.now().AddDate(0, 0, 30), false},
				{friendlyLifetimeName(picoshare.NewFileLifetimeInYears(1), l), s.now().AddDate(1, 0, 0), false},
				{friendlyLifetimeName(picoshare.FileLifetimeInfinite, l), time.Time(picoshare.NeverExpire), true},
			},
			FileLifetimeOptions: []fileLifetimeOption{
				{picoshare.NewFileLifetimeInDays(1), false},
				{picoshare.NewFileLifetimeInDays(7), false},
				{picoshare.NewFileLifetimeInDays(30), false},
				{picoshare.NewFileLifetimeInYears(1), false},
				{picoshare.FileLifetimeInfinite, true},
			},
		})
	}
}

func (s Server) fileIndexGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format(time.DateOnly)
		},
		"formatExpiration": func(et picoshare.ExpirationTime, l i18n.Localizer) string {
			if et == picoshare.NeverExpire {
				return l.T("lifetime.never")
			}
			t := et.Time().Local()
			delta := t.Sub(s.now())
			daysRemaining := delta.Hours() / 24
			return l.T("expiration.withDays", t.Format(time.DateOnly), daysRemaining)
		},
		"formatFileSize": humanReadableFileSize,
		"isExpiringSoon": func(et picoshare.ExpirationTime) bool {
			return isExpiringSoon(et, s.now())
		},
		"fileTypeIcon": fileTypeIcon,
	}

	t := parseTemplatesWithFuncs(fns, "templates/pages/file-index.html")

	return s.fileIndexGetWithOptions(t, "title.files", false, func(ctx context.Context) []store.ReadEntriesOption {
		user, _ := currentUser(ctx)
		return []store.ReadEntriesOption{store.FilterByOwner(user.ID)}
	})
}

// fileAllGet shows every entry regardless of owner. Only administrators can
// reach this route.
func (s Server) fileAllGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format(time.DateOnly)
		},
		"formatExpiration": func(et picoshare.ExpirationTime, l i18n.Localizer) string {
			if et == picoshare.NeverExpire {
				return l.T("lifetime.never")
			}
			t := et.Time().Local()
			delta := t.Sub(s.now())
			daysRemaining := delta.Hours() / 24
			return l.T("expiration.withDays", t.Format(time.DateOnly), daysRemaining)
		},
		"formatFileSize": humanReadableFileSize,
		"isExpiringSoon": func(et picoshare.ExpirationTime) bool {
			return isExpiringSoon(et, s.now())
		},
		"fileTypeIcon": fileTypeIcon,
	}

	t := parseTemplatesWithFuncs(fns, "templates/pages/file-index.html")

	return s.fileIndexGetWithOptions(t, "title.allFiles", true, func(context.Context) []store.ReadEntriesOption {
		return nil
	})
}

func (s Server) fileIndexGetWithOptions(t *template.Template, titleKey string, showOwner bool, optsFromContext func(context.Context) []store.ReadEntriesOption) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		em, err := s.store.GetEntriesMetadata(optsFromContext(r.Context())...)
		if err != nil {
			log.Printf("failed to retrieve entries metadata: %v", err)
			http.Error(w, "failed to retrieve file index", http.StatusInternalServerError)
			return
		}
		sort.Slice(em, func(i, j int) bool {
			return em[i].Uploaded.After(em[j].Uploaded)
		})
		renderTemplate(w, t, struct {
			commonProps
			Files     []picoshare.UploadMetadata
			ShowOwner bool
		}{
			commonProps: makeCommonProps(titleKey, r.Context()),
			Files:       em,
			ShowOwner:   showOwner,
		})
	}
}

func (s Server) fileEditGet() http.HandlerFunc {
	fns := template.FuncMap{
		"isNeverExpire": func(et picoshare.ExpirationTime) bool {
			return et == picoshare.NeverExpire
		},
		"formatExpiration": func(et picoshare.ExpirationTime, l i18n.Localizer) string {
			if et == picoshare.NeverExpire {
				return l.T("lifetime.never")
			}
			return time.Time(et).Format(time.RFC3339)
		},
	}

	t := parseTemplatesWithFuncs(fns,
		"templates/custom-elements/expiration-picker.html",
		"templates/pages/file-edit.html")

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
		if err != nil {
			log.Printf("error parsing ID: %v", err)
			http.Error(w, fmt.Sprintf("bad entry ID: %v", err), http.StatusBadRequest)
			return
		}

		metadata, ok := s.manageableEntry(w, r, id)
		if !ok {
			return
		}

		renderTemplate(w, t, struct {
			commonProps
			Metadata            picoshare.UploadMetadata
			MaxPassphraseLength int
		}{
			commonProps:         makeCommonProps("title.fileEdit", r.Context()),
			Metadata:            metadata,
			MaxPassphraseLength: picoshare.MaxPassphraseLength,
		})
	}
}

func (s Server) fileInfoGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatExpiration": func(et picoshare.ExpirationTime, l i18n.Localizer) string {
			if et == picoshare.NeverExpire {
				return l.T("lifetime.never")
			}
			t := et.Time().Local()
			delta := t.Sub(s.now())
			daysRemaining := delta.Hours() / 24
			return l.T("expiration.withDays", t.Format(time.DateOnly), daysRemaining)
		},
		"formatTimestamp": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"formatFileSize": humanReadableFileSize,
	}

	t := parseTemplatesWithFuncs(
		fns,
		"templates/custom-elements/upload-link-box.html",
		"templates/custom-elements/upload-links.html",
		"templates/pages/file-info.html")

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
		if err != nil {
			log.Printf("error parsing ID: %v", err)
			http.Error(w, fmt.Sprintf("bad entry ID: %v", err), http.StatusBadRequest)
			return
		}

		metadata, ok := s.manageableEntry(w, r, id)
		if !ok {
			return
		}

		downloads, err := s.store.GetEntryDownloads(id)
		if err != nil {
			log.Printf("error retrieving downloads for id %v: %v", id, err)
			http.Error(w, "failed to retrieve downloads", http.StatusInternalServerError)
			return
		}

		renderTemplate(w, t, struct {
			commonProps
			Metadata      picoshare.UploadMetadata
			DownloadCount int
		}{
			commonProps:   makeCommonProps("title.fileInfo", r.Context()),
			Metadata:      metadata,
			DownloadCount: len(downloads),
		})
	}
}

func (s Server) fileDownloadsGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatDownloadIndex": func(i, total int) int {
			return total - i
		},
		"formatDownloadTime": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
	}
	t := parseTemplatesWithFuncs(fns, "templates/pages/file-downloads.html")

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
		if err != nil {
			log.Printf("error parsing ID: %v", err)
			http.Error(w, fmt.Sprintf("bad entry ID: %v", err), http.StatusBadRequest)
			return
		}

		metadata, ok := s.manageableEntry(w, r, id)
		if !ok {
			return
		}

		downloads, err := s.store.GetEntryDownloads(id)
		if err != nil {
			log.Printf("error retrieving downloads for id %v: %v", id, err)
			http.Error(w, "failed to retrieve downloads", http.StatusInternalServerError)
			return
		}

		showUniqueOnly := r.URL.Query().Get("unique") == "true"

		filteredDownloads := downloads
		if showUniqueOnly {
			seen := make(map[string]bool)
			var uniqueDownloads []picoshare.DownloadRecord

			for _, download := range downloads {
				if !seen[download.ClientIP] {
					seen[download.ClientIP] = true
					uniqueDownloads = append(uniqueDownloads, download)
				}
			}
			filteredDownloads = uniqueDownloads
		}

		// Convert raw downloads to display-friendly information.
		type downloadRecord struct {
			Time     time.Time
			ClientIP string
			Browser  string
			Platform string
		}
		records := make([]downloadRecord, len(filteredDownloads))
		for i, d := range filteredDownloads {
			agent := useragent.Parse(d.UserAgent)
			records[i] = downloadRecord{
				Time:     d.Time,
				ClientIP: d.ClientIP,
				Browser:  agent.Name,
				Platform: agent.OS,
			}
		}

		renderTemplate(w, t, struct {
			commonProps
			Metadata       picoshare.UploadMetadata
			Downloads      []downloadRecord
			ShowUniqueOnly bool
		}{
			commonProps:    makeCommonProps("title.fileDownloads", r.Context()),
			Metadata:       metadata,
			Downloads:      records,
			ShowUniqueOnly: showUniqueOnly,
		})
	}
}

func (s Server) fileConfirmDeleteGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/file-delete.html")

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
		if err != nil {
			log.Printf("error parsing ID: %v", err)
			http.Error(w, fmt.Sprintf("bad entry ID: %v", err), http.StatusBadRequest)
			return
		}

		metadata, ok := s.manageableEntry(w, r, id)
		if !ok {
			return
		}
		renderTemplate(w, t, struct {
			commonProps
			Metadata picoshare.UploadMetadata
		}{
			commonProps: makeCommonProps("title.fileDelete", r.Context()),
			Metadata:    metadata,
		})
	}
}

// authPageProps is the data templates/pages/auth.html renders. authGet and
// oidcCallbackGet (which re-renders the login page after a failed login)
// share it so both pass the template the same set of fields.
type authPageProps struct {
	commonProps
	NeedsSetup      bool
	DevLoginEnabled bool
	LoginError      string
}

// authPageProps builds the login page's data. loginErrorKey is an i18n
// message key, or empty for no error.
func (s Server) authPageProps(ctx context.Context, loginErrorKey string) (authPageProps, error) {
	needsSetup, err := s.store.NeedsSetup()
	if err != nil {
		return authPageProps{}, err
	}
	loginError := ""
	if loginErrorKey != "" {
		loginError = localizerFromContext(ctx).T(loginErrorKey)
	}
	return authPageProps{
		commonProps:     makeCommonProps("title.login", ctx),
		NeedsSetup:      needsSetup,
		DevLoginEnabled: devLoginEnabled,
		LoginError:      loginError,
	}, nil
}

func (s Server) authGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/auth.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticated(r.Context()) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		var loginErrorKey string
		if r.URL.Query().Get("error") == "not_configured" {
			loginErrorKey = "auth.notConfigured"
		}

		props, err := s.authPageProps(r.Context(), loginErrorKey)
		if err != nil {
			log.Printf("failed to check setup status: %v", err)
			http.Error(w, "Failed to check setup status", http.StatusInternalServerError)
			return
		}

		renderTemplate(w, t, props)
	}
}

func (s Server) uploadGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatExpiration": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format(time.RFC3339)
		},
	}

	t := parseTemplatesWithFuncs(
		fns,
		"templates/custom-elements/expiration-picker.html",
		"templates/custom-elements/upload-link-box.html",
		"templates/custom-elements/upload-links.html",
		"templates/pages/upload.html")

	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.store.ReadSettings()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read settings from database: %v", err), http.StatusInternalServerError)
			return
		}
		l := localizerFromContext(r.Context())
		type lifetimeOption struct {
			Lifetime  picoshare.FileLifetime
			IsDefault bool
		}
		lifetimeOptions := []lifetimeOption{
			{picoshare.NewFileLifetimeInDays(1), false},
			{picoshare.NewFileLifetimeInDays(7), false},
			{picoshare.NewFileLifetimeInDays(30), false},
			{picoshare.NewFileLifetimeInYears(1), false},
			{picoshare.FileLifetimeInfinite, false},
		}

		defaultIsBuiltIn := false
		for i, lto := range lifetimeOptions {
			if lto.Lifetime.Equal(settings.DefaultFileLifetime) {
				lifetimeOptions[i].IsDefault = true
				defaultIsBuiltIn = true
			}
		}
		// If the default isn't one of the built-in options, add it and sort the
		// list.
		if !defaultIsBuiltIn {
			lifetimeOptions = append(lifetimeOptions, lifetimeOption{settings.DefaultFileLifetime, true})
			sort.Slice(lifetimeOptions, func(i, j int) bool {
				return lifetimeOptions[i].Lifetime.LessThan(lifetimeOptions[j].Lifetime)
			})
		}

		type expirationOption struct {
			FriendlyName string
			Expiration   time.Time
			IsDefault    bool
		}
		expirationOptions := []expirationOption{}
		for _, lto := range lifetimeOptions {
			friendlyName := friendlyLifetimeName(lto.Lifetime, l)
			expiration := lto.Lifetime.ExpirationFromTime(s.now())
			if lto.Lifetime.Equal(picoshare.FileLifetimeInfinite) {
				expiration = picoshare.NeverExpire
			}
			expirationOptions = append(expirationOptions, expirationOption{
				FriendlyName: friendlyName,
				Expiration:   expiration.Time(),
				IsDefault:    lto.IsDefault,
			})
		}

		expirationOptions = append(expirationOptions, expirationOption{l.T("lifetime.custom"), time.Time{}, false})

		renderTemplate(w, t, struct {
			commonProps
			ExpirationOptions   []expirationOption
			MaxNoteLength       int
			MaxPassphraseLength int
			GuestLinkMetadata   picoshare.GuestLink
		}{
			commonProps:         makeCommonProps("title.upload", r.Context()),
			MaxNoteLength:       parse.MaxFileNoteBytes,
			MaxPassphraseLength: picoshare.MaxPassphraseLength,
			ExpirationOptions:   expirationOptions,
		})
	}
}

func (s Server) guestUploadGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatExpiration": func(t time.Time) string {
			return t.Format(time.RFC3339)
		}}

	t := parseTemplatesWithFuncs(
		fns,
		"templates/custom-elements/expiration-picker.html",
		"templates/custom-elements/upload-link-box.html",
		"templates/custom-elements/upload-links.html",
		"templates/pages/upload.html")

	tInactive := parseTemplates("templates/pages/guest-link-inactive.html")

	return func(w http.ResponseWriter, r *http.Request) {
		guestLinkID, err := parseGuestLinkID(mux.Vars(r)["guestLinkID"])
		if err != nil {
			log.Printf("error parsing guest link ID: %v", err)
			http.Error(w, fmt.Sprintf("Invalid guest link ID: %v", err), http.StatusBadRequest)
			return
		}

		gl, err := s.store.GetGuestLink(guestLinkID)
		if _, ok := errors.AsType[store.GuestLinkNotFoundError](err); ok {
			http.Error(w, "Invalid guest link ID", http.StatusNotFound)
			return
		} else if err != nil {
			log.Printf("error retrieving guest link with ID %v: %v", guestLinkID, err)
			http.Error(w, "Failed to retrieve guest link", http.StatusInternalServerError)
			return
		}

		if !gl.IsActive() {
			renderTemplate(w, tInactive, struct {
				commonProps
			}{
				commonProps: makeCommonProps("title.guestLinkInactive", r.Context()),
			})
			return
		}

		// Generate expiration options up to the guest link's maximum file lifetime.
		type lifetimeOption struct {
			Lifetime  picoshare.FileLifetime
			IsDefault bool
		}
		type expirationOption struct {
			FriendlyName string
			Expiration   time.Time
			IsDefault    bool
		}

		baseLifetimeOptions := []lifetimeOption{
			{picoshare.NewFileLifetimeInDays(1), false},
			{picoshare.NewFileLifetimeInDays(7), false},
			{picoshare.NewFileLifetimeInDays(30), false},
			{picoshare.NewFileLifetimeInYears(1), false},
			{picoshare.FileLifetimeInfinite, false},
		}

		// Filter options to only include those within the guest link's maximum.
		validLifetimeOptions := []lifetimeOption{}
		for _, lto := range baseLifetimeOptions {
			if lto.Lifetime.Days() <= gl.MaxFileLifetime.Days() {
				validLifetimeOptions = append(validLifetimeOptions, lto)
			}
		}

		// Mark the guest link's file lifetime as the default.
		for i, lto := range validLifetimeOptions {
			if lto.Lifetime.Equal(gl.MaxFileLifetime) {
				validLifetimeOptions[i].IsDefault = true
				break
			}
		}

		// Convert to expiration options.
		l := localizerFromContext(r.Context())
		expirationOptions := []expirationOption{}
		for _, lto := range validLifetimeOptions {
			friendlyName := friendlyLifetimeName(lto.Lifetime, l)
			expiration := lto.Lifetime.ExpirationFromTime(s.now())
			if lto.Lifetime.Equal(picoshare.FileLifetimeInfinite) {
				expiration = picoshare.NeverExpire
			}
			expirationOptions = append(expirationOptions, expirationOption{
				FriendlyName: friendlyName,
				Expiration:   expiration.Time(),
				IsDefault:    lto.IsDefault,
			})
		}

		renderTemplate(w, t, struct {
			commonProps
			ExpirationOptions []expirationOption
			GuestLinkMetadata picoshare.GuestLink
		}{
			commonProps:       makeCommonProps("title.upload", r.Context()),
			ExpirationOptions: expirationOptions,
			GuestLinkMetadata: gl,
		})
	}
}

func (s Server) settingsGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/settings.html")

	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.store.ReadSettings()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read settings from database: %v", err), http.StatusInternalServerError)
			return
		}
		var defaultExpiration uint16
		var expirationTimeUnit string
		defaultNeverExpire := settings.DefaultFileLifetime.Equal(picoshare.FileLifetimeInfinite)
		if defaultNeverExpire {
			defaultExpiration = 30
			expirationTimeUnit = "days"
		} else {
			defaultExpiration = settings.DefaultFileLifetime.Days()
			expirationTimeUnit = "days"
			if settings.DefaultFileLifetime.IsYearBoundary() {
				defaultExpiration = settings.DefaultFileLifetime.Years()
				expirationTimeUnit = "years"
			}
		}

		// Suggest a sensible retention period when the user switches from keeping
		// download history forever.
		downloadHistoryRetentionDays := uint16(90)
		keepDownloadHistoryForever := settings.DownloadHistoryRetention.IsForever()
		if !keepDownloadHistoryForever {
			downloadHistoryRetentionDays = settings.DownloadHistoryRetention.Days()
		}

		renderTemplate(w, t, struct {
			commonProps
			DefaultExpiration               uint16
			ExpirationTimeUnit              string
			DefaultNeverExpire              bool
			DownloadHistoryRetentionDays    uint16
			MaxDownloadHistoryRetentionDays uint16
			KeepDownloadHistoryForever      bool
			DefaultLanguage                 string
		}{
			commonProps:                     makeCommonProps("title.settings", r.Context()),
			DefaultExpiration:               defaultExpiration,
			ExpirationTimeUnit:              expirationTimeUnit,
			DefaultNeverExpire:              defaultNeverExpire,
			DownloadHistoryRetentionDays:    downloadHistoryRetentionDays,
			MaxDownloadHistoryRetentionDays: picoshare.MaxDownloadHistoryRetentionDays,
			KeepDownloadHistoryForever:      keepDownloadHistoryForever,
			DefaultLanguage:                 settings.DefaultLanguage.String(),
		})
	}
}

func (s Server) systemInformationGet() http.HandlerFunc {
	fns := template.FuncMap{
		"formatDiskUsage": humanReadableDiskUsage,
		"percentage": func(part, total uint64) string {
			return fmt.Sprintf("%.0f%%", 100.0*(float64(part)/float64(total)))
		},
	}
	t := parseTemplatesWithFuncs(fns, "templates/pages/system-information.html")

	return func(w http.ResponseWriter, r *http.Request) {
		spaceUsage, err := s.checkSpace()
		if err != nil {
			log.Printf("error checking available space: %v", err)
			http.Error(w, fmt.Sprintf("failed to check available space: %v", err), http.StatusInternalServerError)
			return
		}

		reclaimableBytes, err := s.store.ReclaimableBytes()
		if err != nil {
			log.Printf("error measuring reclaimable database space: %v", err)
			http.Error(w, "Failed to measure reclaimable database space", http.StatusInternalServerError)
			return
		}

		lastCleanup := s.collector.LastRun()

		renderTemplate(w, t, struct {
			commonProps
			TotalServingBytes uint64
			DatabaseFileBytes uint64
			ReclaimableBytes  uint64
			UsedBytes         uint64
			TotalBytes        uint64
			LastCleanupTime   time.Time
			LastCleanupFailed bool
			BuildTime         time.Time
			Version           string
			Revision          string
		}{
			commonProps:       makeCommonProps("title.systemInformation", r.Context()),
			TotalServingBytes: spaceUsage.TotalServingBytes,
			DatabaseFileBytes: spaceUsage.DatabaseFileSize,
			ReclaimableBytes:  reclaimableBytes,
			UsedBytes:         spaceUsage.FileSystemUsedBytes,
			TotalBytes:        spaceUsage.FileSystemTotalBytes,
			LastCleanupTime:   lastCleanup.Time,
			LastCleanupFailed: lastCleanup.Err != nil,
			BuildTime:         build.Time(),
			Version:           build.Version(),
			Revision:          build.Revision(),
		})
	}
}

func humanReadableFileSize(fileSize picoshare.FileSize) string {
	return humanReadableDiskUsage(fileSize.UInt64())
}

// friendlyLifetimeName renders lt as localized display text, such as "3
// days" or "永不過期". It lives here, rather than on the domain type, because
// the domain layer doesn't produce user-facing strings.
func friendlyLifetimeName(lt picoshare.FileLifetime, l i18n.Localizer) string {
	if lt.Equal(picoshare.FileLifetimeInfinite) {
		return l.T("lifetime.never")
	}
	if lt.IsYearBoundary() {
		years := lt.Years()
		if years == 1 {
			return l.T("lifetime.year", years)
		}
		return l.T("lifetime.years", years)
	}
	days := lt.Days()
	if days == 1 {
		return l.T("lifetime.day", days)
	}
	return l.T("lifetime.days", days)
}

// expiringSoonThreshold is how close to its expiration time an entry must be
// before the file list flags it with a warning badge.
const expiringSoonThreshold = 72 * time.Hour

// isExpiringSoon reports whether et falls within expiringSoonThreshold of
// now, excluding entries that never expire or have already expired (the
// cleanup job removes those before a user would see them here).
func isExpiringSoon(et picoshare.ExpirationTime, now time.Time) bool {
	if et == picoshare.NeverExpire {
		return false
	}
	delta := et.Time().Sub(now)
	return delta > 0 && delta <= expiringSoonThreshold
}

// fileTypeIcon returns the Font Awesome icon class that best represents ct.
func fileTypeIcon(ct picoshare.ContentType) string {
	s := ct.String()
	switch {
	case strings.HasPrefix(s, "image/"):
		return "fa-file-image"
	case strings.HasPrefix(s, "video/"):
		return "fa-file-video"
	case strings.HasPrefix(s, "audio/"):
		return "fa-file-audio"
	case strings.HasPrefix(s, "text/"):
		return "fa-file-lines"
	case s == "application/pdf":
		return "fa-file-pdf"
	case s == "application/zip", s == "application/x-7z-compressed", s == "application/x-tar", s == "application/gzip", s == "application/x-rar-compressed":
		return "fa-file-zipper"
	case strings.Contains(s, "word"):
		return "fa-file-word"
	case strings.Contains(s, "excel") || strings.Contains(s, "spreadsheet"):
		return "fa-file-excel"
	default:
		return "fa-file"
	}
}

// guestLinkUploadProgressPercent returns how full a guest link's upload
// allowance is, from 0 to 100, or -1 when the guest link has no upload
// limit (in which case the caller shouldn't show a progress bar at all).
func guestLinkUploadProgressPercent(uploaded int, limit picoshare.GuestUploadCountLimit) int {
	if limit == picoshare.GuestUploadUnlimitedFileUploads {
		return -1
	}
	if *limit <= 0 {
		return 100
	}
	pct := int(100 * float64(uploaded) / float64(*limit))
	if pct > 100 {
		return 100
	}
	return pct
}

func humanReadableDiskUsage(b uint64) string {
	const unit = 1024

	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// makeCommonProps builds the template data every page needs. titleKey is an
// i18n message key, not literal text, so the rendered title follows the
// request's resolved language.
func makeCommonProps(titleKey string, ctx context.Context) commonProps {
	user, ok := currentUser(ctx)
	l := localizerFromContext(ctx)
	return commonProps{
		Title:           l.T(titleKey),
		IsAuthenticated: ok,
		IsAdmin:         user.IsAdmin,
		Username:        user.Username.String(),
		CspNonce:        cspNonce(ctx),
		L:               l,
		Lang:            l.Lang(),
		SupportedLangs:  i18n.SupportedLanguages,
	}
}

func renderTemplate(w http.ResponseWriter, t *template.Template, data any) {
	// Render the complete page before writing any bytes so execution failures do
	// not commit a successful status with a partial response body.
	var rendered bytes.Buffer
	if err := t.Execute(&rendered, data); err != nil {
		log.Printf("failed to render template %q: %v", t.Name(), err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	if _, err := rendered.WriteTo(w); err != nil {
		log.Printf("failed to write rendered template %q: %v", t.Name(), err)
	}
}

func parseTemplates(templatePaths ...string) *template.Template {
	return parseTemplatesWithFuncs(template.FuncMap{}, templatePaths...)
}

// baseFuncs are available to every template, on top of whatever
// page-specific functions parseTemplatesWithFuncs is called with.
var baseFuncs = template.FuncMap{
	// toJSON embeds a Go value as a JSON literal in a template, such as the
	// client-side translations base.html embeds for static/js/lib/i18n.js.
	"toJSON": func(v any) (template.JS, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(b), nil
	},
	// dict builds a map from alternating string keys and values, for passing
	// more than one value into a {{ template }} invocation, which otherwise
	// only accepts a single pipeline argument.
	"dict": func(pairs ...any) (map[string]any, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict requires an even number of arguments")
		}
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings, got %T", pairs[i])
			}
			m[key] = pairs[i+1]
		}
		return m, nil
	},
}

func parseTemplatesWithFuncs(fns template.FuncMap, templatePaths ...string) *template.Template {
	merged := template.FuncMap{}
	for k, v := range baseFuncs {
		merged[k] = v
	}
	for k, v := range fns {
		merged[k] = v
	}
	return template.Must(
		template.New("base.html").
			Funcs(merged).
			ParseFS(
				templatesFS,
				append(
					[]string{
						"templates/layouts/base.html",
						"templates/partials/navbar.html",
						"templates/custom-elements/snackbar-notifications.html",
					},
					templatePaths...)...))
}
