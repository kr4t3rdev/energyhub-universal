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
		// No se pudo extraer la contraseña (URL malformada o formato
		// clave=valor de libpq que url.Parse no entiende). No se devuelve el
		// error crudo: podría contener la contraseña y terminar en los logs de
		// arranque.
		return errors.New("database connection failed: unable to redact credentials")
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
