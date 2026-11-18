package postgres

import (
	"errors"
	"net/url"
	"strings"
)

// RedactedPassword is what replaces a password in a connection error.
const RedactedPassword = "xxxxx"

// RedactConnectionError removes the password of a connection string from an
// error message. The database drivers make a best effort redaction of their
// own, but they do not guarantee it for a malformed connection string, and
// the message ends up in the startup log.
func RedactConnectionError(err error, databaseURL string) error {
	if err == nil {
		return nil
	}
	password := connectionPassword(databaseURL)
	if password == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), password, RedactedPassword))
}

// connectionPassword returns the password of a connection string, or an empty
// string when it has none.
func connectionPassword(databaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(databaseURL))
	if err != nil || parsed.User == nil {
		return ""
	}
	password, set := parsed.User.Password()
	if !set {
		return ""
	}
	return password
}
