package constants

import (
	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	ExitCodeEditMappings             gaba.ExitCode = 100
	ExitCodeSaveSync                 gaba.ExitCode = 101
	ExitCodeInfo                     gaba.ExitCode = 102
	ExitCodeLogout                   gaba.ExitCode = 103
	ExitCodeBIOS                     gaba.ExitCode = 104
	ExitCodeLogoutConfirm            gaba.ExitCode = 105
	ExitCodeSyncArtwork              gaba.ExitCode = 106
	ExitCodeCollectionsSettings      gaba.ExitCode = 107
	ExitCodeAdvancedSettings         gaba.ExitCode = 108
	ExitCodeRefreshCache             gaba.ExitCode = 111
	ExitCodeSaveSyncSettings         gaba.ExitCode = 112
	ExitCodeGameOptions              gaba.ExitCode = 113
	ExitCodeGeneralSettings          gaba.ExitCode = 114
	ExitCodeCheckUpdate              gaba.ExitCode = 115
	ExitCodeDownloadRequested        gaba.ExitCode = 116
	ExitCodeSearch                   gaba.ExitCode = 200
	ExitCodeClearSearch              gaba.ExitCode = 201
	ExitCodeCollections              gaba.ExitCode = 300
	ExitCodeBackToCollection         gaba.ExitCode = 301
	ExitCodeBackToCollectionPlatform gaba.ExitCode = 302
	ExitCodeNoResults                gaba.ExitCode = 404
)
