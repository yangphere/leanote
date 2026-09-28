package lea

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"
)

// Log/Logf/LogW/LogJ are the project-wide logging facade used by services.
// The native entrypoint owns the process logger; this package keeps a small
// standard-library seam so library code remains usable in tests.
func Log(msg string, i ...interface{}) {
	writeLogLine(fmt.Sprint(append([]interface{}{msg}, i...)...))
}

// Logf keeps printf semantics for callers that pass directives.
func Logf(msg string, i ...interface{}) {
	writeLogLine(fmt.Sprintf(msg, i...))
}

func LogW(msg string, i ...interface{}) {
	writeLogLine(fmt.Sprint(append([]interface{}{msg}, i...)...))
}

func LogJ(i interface{}) {
	b, _ := json.MarshalIndent(i, "", " ")
	writeLogLine(string(b))
}

// writeLogLine emits one already-formatted line; it is deliberately
// non-formatting so vet does not treat the facade as a printf wrapper.
func writeLogLine(line string) {
	stdlog.New(os.Stderr, "", stdlog.LstdFlags).Output(2, line)
}
