package environment

import "os"

func IsDevelopment() bool {
	return os.Getenv("ENVIRONMENT") == "DEV"
}

// MenuOnL2 reports whether the device has no Menu button and L2 stands in for it.
// The launch script sets MENU_ON_L2=1 on those devices.
func MenuOnL2() bool {
	return os.Getenv("MENU_ON_L2") == "1"
}
