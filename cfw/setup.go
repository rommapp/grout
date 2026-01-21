package cfw

import (
	"grout/cfw/knulli"
)

// FirstLaunchSetup performs any first-launch setup required by the CFW.
func FirstLaunchSetup() {
	if GetCFW() == Knulli {
		knulli.FirstLaunchSetup(GetRomDirectory())
	}
}
