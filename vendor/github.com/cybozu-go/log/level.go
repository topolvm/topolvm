package log

var (
	severityMap = map[int]string{
		LvCritical: "critical",
		LvError:    "error",
		LvWarn:     "warning",
		LvInfo:     "info",
		LvDebug:    "debug",
	}
)

// LevelName returns the name for the defined threshold.
// An empty string is returned for undefined thresholds.
func LevelName(level int) string {
	return severityMap[level]
}
