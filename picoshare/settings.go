package picoshare

import "fmt"

type Settings struct {
	DefaultFileLifetime      FileLifetime
	DownloadHistoryRetention DownloadHistoryRetention
}

func (s Settings) String() string {
	return fmt.Sprintf("{lifetime=%s, downloadHistoryRetention=%s}", s.DefaultFileLifetime.FriendlyName(), s.DownloadHistoryRetention)
}
