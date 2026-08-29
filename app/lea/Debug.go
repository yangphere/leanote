package lea

import (
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"

	"github.com/revel/revel"
)

// Log/Logf/LogW/LogJ are the project-wide logging facade. Under Revel they
// delegate to revel.AppLog; in plain-Go processes (cmd/leanote, unit tests)
// revel.AppLog is nil and the standard logger is used instead. This is the
// logger seam called out in the C-b design (§1.1).
func Log(msg string, i ...interface{}) {
	if revel.AppLog != nil {
		revel.AppLog.Info(msg, i...)
		return
	}
	// Non-formatting on purpose: legacy callers pass values, not verbs, and
	// vet must not infer a printf wrapper here.
	writeLogLine(fmt.Sprint(append([]interface{}{msg}, i...)...))
}

// Logf keeps printf semantics for callers that pass directives.
func Logf(msg string, i ...interface{}) {
	if revel.AppLog != nil {
		revel.AppLog.Infof(msg, i...)
		return
	}
	writeLogLine(fmt.Sprintf(msg, i...))
}

func LogW(msg string, i ...interface{}) {
	if revel.AppLog != nil {
		revel.AppLog.Warn(msg, i...)
		return
	}
	writeLogLine(fmt.Sprint(append([]interface{}{msg}, i...)...))
}

func LogJ(i interface{}) {
	b, _ := json.MarshalIndent(i, "", " ")
	if revel.AppLog != nil {
		revel.AppLog.Info(string(b))
		return
	}
	writeLogLine(string(b))
}

// writeLogLine emits one already-formatted line; it is deliberately
// non-formatting so vet does not treat the facade as a printf wrapper.
func writeLogLine(line string) {
	stdlog.New(os.Stderr, "", stdlog.LstdFlags).Output(2, line)
}
