package service

// BuildVersion is injected for release builds with go link flags. Keeping the
// development default explicit avoids a second hard-coded production version.
var BuildVersion = "dev"
