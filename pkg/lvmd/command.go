package lvmd

import (
	internalLvmdCommand "github.com/topolvm/topolvm/internal/lvmd/command"
)

func Containerized(sw bool) {
	internalLvmdCommand.Containerized = sw
}
