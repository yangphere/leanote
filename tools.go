//go:build tools

// Package tools exists only to keep runtime-resolved Revel modules in the
// module graph. conf/app.conf declares `module.static`, which Revel resolves
// by import path at build/startup; without a requirement, `go mod tidy`
// prunes it and module loading fails with "Failed to load module". Never
// built by default (the `tools` tag is unset everywhere).
package tools

import (
	_ "github.com/bradfitz/gomemcache/memcache"
	_ "github.com/garyburd/redigo/redis"
	_ "github.com/patrickmn/go-cache"
	_ "github.com/revel/modules/static"
)
