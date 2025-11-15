package clients

import (
	"grout/models"

	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

func BuildClient(host models.Host) (shared.Client, error) {
	return NewRomMClient(
		host.RootURI,
		host.Port,
		host.Username,
		host.Password,
	), nil
}
