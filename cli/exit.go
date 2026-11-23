package cli

import "github.com/rommapp/grout/api"

type ExitCode int

const (
	ExitOK            ExitCode = 0
	ExitUsage         ExitCode = 1
	ExitServer        ExitCode = 2
	ExitPending       ExitCode = 3
	ExitTooOld        ExitCode = 4
	ExitNotConfigured ExitCode = 5
	ExitUnauthorized  ExitCode = 6
	ExitBusy          ExitCode = 7
	ExitLocked        ExitCode = 8
	ExitCancelled     ExitCode = 9
)

func exitFor(c api.Code) ExitCode {
	switch c {
	case "":
		return ExitOK
	case api.CodeBadRequest:
		return ExitUsage
	case api.CodeServerUnreachable, api.CodeStorage, api.CodeInternal, api.CodeBadArchive, api.CodeMissingOnServer:
		return ExitServer
	case api.CodeServerTooOld:
		return ExitTooOld
	case api.CodeNotConfigured, api.CodeBadDefinition:
		return ExitNotConfigured
	case api.CodeUnauthorized, api.CodeInvalidCode:
		return ExitUnauthorized
	case api.CodeBusy, api.CodeRateLimited:
		return ExitBusy
	case api.CodeDataDirLocked:
		return ExitLocked
	case api.CodeCancelled:
		return ExitCancelled
	}
	return ExitServer
}
