// Package constants holds application-wide constant values.
package constants

const (
	// HeaderAuthorization is the metadata/header key carrying bearer tokens.
	HeaderAuthorization = "authorization"
	// HeaderRequestID propagates the request id across services.
	HeaderRequestID = "x-request-id"

	// BearerPrefix is the expected authorization scheme prefix.
	BearerPrefix = "Bearer "
)
