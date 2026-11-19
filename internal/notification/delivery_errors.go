package notification

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// A webhook or Slack URL is the credential: its path, query or userinfo carries the token. Go quotes the
// request URL in a *url.Error, and a bad redirect quotes more of them in the inner error, so the text of
// such an error is never kept. requestError names the operation and the failure class instead.
func requestError(what string, err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", what, err) // an invalid method: the text names no URL
	}
	class := "failed"
	switch {
	case ue.Timeout():
		class = "timed out"
	case errors.Is(err, context.Canceled):
		class = "canceled"
	case errors.Is(err, syscall.ECONNREFUSED):
		class = "connection refused"
	default:
		var dns *net.DNSError
		if errors.As(err, &dns) && dns.IsNotFound { // a SERVFAIL or a refused query stays "failed"
			class = "host not found"
		}
	}
	return fmt.Errorf("%s: %s %s", what, ue.Op, class)
}

// legacyDeliveryErrorPrefixes are the four texts a failed send began with before requestError: each one
// wrapped the quoted request URL. They are never written any more, so a stored error that starts with one
// is an old row.
var legacyDeliveryErrorPrefixes = []string{
	"failed to send webhook:",
	"failed to send slack notification:",
	"failed to create request:",
	"failed to create slack request:",
}

// RemoveLegacyDeliveryErrors replaces the whole text of each old failed-send row with its prefix and a
// note, so a stored URL is no longer readable through the history. It matches the exact prefix and never
// the URL, and keeps nothing of the old text. It returns the number of rows changed, and a second run
// changes none. NewService runs it at every start: four LIKE scans of the history table are the cost.
func RemoveLegacyDeliveryErrors(db *gorm.DB) (int64, error) {
	if !db.Migrator().HasTable(&NotificationHistory{}) {
		return 0, nil
	}
	var changed int64
	for _, prefix := range legacyDeliveryErrorPrefixes {
		note := prefix + " details removed"
		col := clause.Column{Name: "error"}
		res := db.Model(&NotificationHistory{}).
			Where(clause.Like{Column: col, Value: prefix + "%"}).
			Where(clause.Neq{Column: col, Value: note}).
			Update("error", note)
		if res.Error != nil {
			return changed, res.Error
		}
		changed += res.RowsAffected
	}
	return changed, nil
}
