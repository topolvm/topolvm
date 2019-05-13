package well

import (
	"io"
	"os"
)

func openLogFile(filename string) (io.Writer, error) {
	return os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
}
