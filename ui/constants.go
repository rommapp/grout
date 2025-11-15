package ui

import (
	"grout/models"

	"qlova.tech/sum"
)

var Screens = sum.Int[models.ScreenName]{}.Sum()
