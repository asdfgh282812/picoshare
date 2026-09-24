package picoshare

import "fmt"

type Settings struct {
	DefaultFileLifetime      FileLifetime
	DownloadHistoryRetention DownloadHistoryRetention
	DefaultLanguage          SiteDefaultLanguage
	// MaxNonAdminFileLifetime caps how long a non-admin user's own uploads may
	// live. FileLifetimeInfinite means no cap.
	MaxNonAdminFileLifetime FileLifetime
}

func (s Settings) String() string {
	return fmt.Sprintf("{lifetime=%s, downloadHistoryRetention=%s, defaultLanguage=%s, maxNonAdminFileLifetime=%s}", s.DefaultFileLifetime, s.DownloadHistoryRetention, s.DefaultLanguage, s.MaxNonAdminFileLifetime)
}
