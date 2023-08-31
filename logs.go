package jobqueue

type LogFlag uint16

const (
	LogFlagNone LogFlag = 0
	LogFlagAll  LogFlag = 0xFFFF
)

const (
	LogManagerStarted LogFlag = 1 << iota
	LogManagerStopped
	LogJobStarted
	LogJobCompleted
	LogJobFailed
	LogWaiting
)

func (m *manager[T, ID]) flagPrintf(flag LogFlag, format string, args ...any) {
	if m.options.LogFlags&flag > 0 {
		m.printf(format, args...)
	}
}

func (m *manager[T, ID]) printf(format string, args ...any) {
	if m.options.Logger != nil {
		m.options.Logger.Printf(format, args...)
	}
}
