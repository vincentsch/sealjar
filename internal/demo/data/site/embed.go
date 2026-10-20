// Package site holds the example bakery's static files.
package site

import "embed"

//go:embed *.html assets prices contact private login account qa
var Files embed.FS
