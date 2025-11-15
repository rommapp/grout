package clients

import (
	"grout/models"

	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

func BuildClient(host models.Host) (shared.Client, error) {
	switch host.HostType {
	case shared.HostTypes.MEGATHREAD:
		return common.NewHttpTableClient(
			host.RootURI,
			host.HostType,
			host.TableColumns,
			host.SourceReplacements,
			nil,
		), nil
	case shared.HostTypes.ROMM:
		{
			return NewRomMClient(
				host.RootURI,
				host.Port,
				host.Username,
				host.Password,
			), nil
		}
	}

	return nil, nil
}
