package picoshare

import "fmt"

type Settings struct {
	DefaultFileLifetime      FileLifetime
	DownloadHistoryRetention DownloadHistoryRetention
	DefaultLanguage          SiteDefaultLanguage
}

func (s Settings) String() string {
	return fmt.Sprintf("{lifetime=%s, downloadHistoryRetention=%s, defaultLanguage=%s}", s.DefaultFileLifetime, s.DownloadHistoryRetention, s.DefaultLanguage)
}
