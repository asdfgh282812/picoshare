package store

import "github.com/mtlynch/picoshare/picoshare"

// ReadEntriesOptions controls which entries GetEntriesMetadata returns.
type ReadEntriesOptions struct {
	OwnerID picoshare.UserID
}

type ReadEntriesOption func(*ReadEntriesOptions)

// FilterByOwner restricts GetEntriesMetadata to entries owned by the given
// user. Without this option, GetEntriesMetadata returns every entry,
// regardless of owner.
func FilterByOwner(id picoshare.UserID) ReadEntriesOption {
	return func(o *ReadEntriesOptions) {
		o.OwnerID = id
	}
}

func ResolveReadEntriesOptions(opts []ReadEntriesOption) ReadEntriesOptions {
	var o ReadEntriesOptions
	for _, apply := range opts {
		apply(&o)
	}
	return o
}
