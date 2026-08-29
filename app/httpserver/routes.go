package httpserver

import (
	"fmt"
	"strings"
)

// Route is one parsed line of conf/routes. Patterns use Revel's syntax:
// ":name" matches one path segment, a trailing "*" (bare or "*name")
// matches the rest of the path. Method is GET/POST/* (upper-cased; the
// file mixes cases).
type Route struct {
	Method     string
	Path       string
	Action     string // "Controller.Method"; empty for static routes
	Controller string
	MethodName string
	StaticBase string // Static.Serve("base") argument
	IsStatic   bool
	IsCatchAll bool // path contains :controller
	segments   []routeSegment
}

type routeSegment struct {
	kind segmentKind
	text string // literal text or param name
}

type segmentKind int

const (
	segLiteral segmentKind = iota
	segParam
	segRest
)

// ParseRoutes parses conf/routes content into an ordered route table. The
// file is priority-ordered: first match wins. Static.Serve("base") entries
// become static routes; "*  /:controller/:action" lines become catch-alls.
func ParseRoutes(data []byte) ([]Route, error) {
	var routes []Route
	for lineNumber, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("routes line %d: could not parse %q", lineNumber+1, line)
		}
		route := Route{
			Method: strings.ToUpper(fields[0]),
			Path:   fields[1],
		}
		action := fields[2]
		if base, ok := parseStaticServe(action); ok {
			route.IsStatic = true
			route.StaticBase = base
			route.Controller = "Static"
			route.MethodName = "Serve"
		} else {
			dot := strings.LastIndex(action, ".")
			if dot <= 0 || dot == len(action)-1 {
				return nil, fmt.Errorf("routes line %d: malformed action %q", lineNumber+1, action)
			}
			// "Blog.E()" — an action declared with an explicit empty
			// argument list — is the same action as "Blog.E".
			route.Action = strings.TrimSuffix(action, "()")
			route.Controller = route.Action[:dot]
			route.MethodName = route.Action[dot+1:]
			if strings.HasSuffix(route.MethodName, "()") {
				route.MethodName = strings.TrimSuffix(route.MethodName, "()")
				route.Action = route.Controller + "." + route.MethodName
			}
		}
		route.IsCatchAll = strings.Contains(route.Path, ":controller")
		route.segments = compileSegments(route.Path)
		routes = append(routes, route)
	}
	return routes, nil
}

// parseStaticServe recognises the Static.Serve("base") action form.
func parseStaticServe(action string) (string, bool) {
	const prefix = `Static.Serve("`
	if !strings.HasPrefix(action, prefix) || !strings.HasSuffix(action, `")`) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(action, prefix), `")`), true
}

func compileSegments(pattern string) []routeSegment {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	segs := make([]routeSegment, 0, len(parts))
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, ":"):
			segs = append(segs, routeSegment{kind: segParam, text: part[1:]})
		case part == "*" || strings.HasPrefix(part, "*"):
			name := strings.TrimPrefix(part, "*")
			if name == "" {
				name = "rest"
			}
			segs = append(segs, routeSegment{kind: segRest, text: name})
		default:
			segs = append(segs, routeSegment{kind: segLiteral, text: part})
		}
	}
	// A trailing bare "*" route ("/blog/*") must also match the prefix
	// itself ("/blog"), which Revel's router accepts.
	return segs
}

func strconvI(i int) string {
	return string(rune('0' + i))
}
