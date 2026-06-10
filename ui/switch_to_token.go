package ui

import (
	"grout/cache"
	"grout/internal"
	"grout/romm"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SwitchToTokenOutput struct {
	Success bool
	Host    romm.Host
}

type SwitchToTokenScreen struct{}

func NewSwitchToTokenScreen() *SwitchToTokenScreen {
	return &SwitchToTokenScreen{}
}

func (s *SwitchToTokenScreen) Execute(config *internal.Config, host romm.Host) SwitchToTokenOutput {
	logger := gabagool.GetLogger()

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_pairing_code", Other: "Pairing Code"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					DisplayName:    "",
					KeyboardPrompt: "",
					Value:          "",
				},
			},
		},
	}

	res, err := gabagool.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_switch_to_token", Other: "Switch to API Token"}, nil),
		gabagool.OptionListSettings{
			FooterHelpItems: []gabagool.FooterHelpItem{
				FooterBack(),
				{ButtonName: icons.LeftRight, HelpText: i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil)},
				{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_continue", Other: "Continue"}, nil)},
			},
			UseSmallTitle: true,
		},
		items,
	)

	if err != nil {
		return SwitchToTokenOutput{}
	}

	code := res.Items[0].Options[0].Value.(string)
	if code == "" {
		return SwitchToTokenOutput{}
	}

	// Exchange the pairing code for a token
	result, _ := gabagool.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "login_validating", Other: "Logging in..."}, nil),
		gabagool.ProcessMessageOptions{},
		func() (interface{}, error) {
			tokenResp, err := romm.ExchangeToken(host.URL(), code, host.InsecureSkipVerify)
			if err != nil {
				return nil, err
			}

			newHost := host
			newHost.Token = tokenResp.RawToken
			newHost.TokenName = tokenResp.Name
			newHost.TokenExpiresAt = tokenResp.ExpiresAt
			newHost.Password = ""

			if missing := romm.MissingSyncScopes(tokenResp.Scopes); len(missing) > 0 {
				logger.Warn("Paired token is missing scopes needed for save sync",
					"missing", missing, "granted", tokenResp.Scopes)
			}

			// Validate the token works
			client := romm.NewClientFromHost(newHost, internal.LoginTimeout)
			if err := client.ValidateToken(); err != nil {
				return nil, err
			}

			// Fetch username if missing
			if newHost.Username == "" {
				if user, err := client.GetCurrentUser(); err == nil {
					newHost.Username = user.Username
				}
			}

			return newHost, nil
		},
	)

	if result == nil {
		logger.Warn("Token exchange failed")
		gabagool.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "login_error_invalid_code", Other: "Invalid or expired pairing code.\nPlease try again."}, nil),
			ContinueFooter(),
			gabagool.MessageOptions{},
		)
		return SwitchToTokenOutput{}
	}

	newHost := result.(romm.Host)

	// Update config
	config.Hosts[0] = newHost
	if err := internal.SaveConfig(config); err != nil {
		logger.Error("Failed to save config after token switch", "error", err)
	}

	// Re-initialize cache manager with new credentials
	if err := cache.InitCacheManager(newHost, config); err != nil {
		logger.Error("Failed to re-initialize cache manager", "error", err)
	}

	gabagool.ConfirmationMessage(
		i18n.Localize(&goi18n.Message{ID: "settings_token_switch_success", Other: "Successfully switched to API Token!"}, nil),
		ContinueFooter(),
		gabagool.MessageOptions{},
	)

	return SwitchToTokenOutput{Success: true, Host: newHost}
}
