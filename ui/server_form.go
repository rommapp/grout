package ui

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	"grout/settings"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// serverForm asks for the address of a RomM server. First-run login and the
// settings screen want exactly the same fields and differ only in what the
// title and buttons say.
type serverForm struct {
	Title  string
	Footer []gabagool.FooterHelpItem
	// SmallTitle and StatusBar suit a screen shown inside the app. The login
	// form runs before there is anything to put in a status bar.
	SmallTitle bool
	StatusBar  gabagool.StatusBarOptions
}

// draw shows the form filled in from host and returns it with the user's
// answers applied. The second result is false when the user backed out, which
// leaves host untouched.
func (f serverForm) draw(host settings.Host) (settings.Host, bool, error) {
	usesHTTPS := strings.HasPrefix(host.RootURI, "https://")

	// The SSL row only makes sense over HTTPS, so it follows the protocol.
	sslVisible := &atomic.Bool{}
	sslVisible.Store(usesHTTPS)

	hostname := removeScheme(host.RootURI)
	port := ""
	if host.Port != 0 {
		port = strconv.Itoa(host.Port)
	}

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{Text: localize("login_protocol", "Protocol")},
			Options: []gabagool.Option{
				{
					DisplayName: localize("login_protocol_http", "HTTP"),
					Value:       "http://",
					OnUpdate:    func(any) { sslVisible.Store(false) },
				},
				{
					DisplayName: localize("login_protocol_https", "HTTPS"),
					Value:       "https://",
					OnUpdate:    func(any) { sslVisible.Store(true) },
				},
			},
			SelectedOption: boolToIndex(usesHTTPS),
		},
		{
			Item: gabagool.MenuItem{Text: localize("login_hostname", "Hostname")},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					KeyboardLayout: gabagool.KeyboardLayoutURL,
					URLShortcuts: []gabagool.URLShortcut{
						{Value: "romm.", SymbolValue: "romm."},
						{Value: ".com", SymbolValue: ".com"},
						{Value: ".org", SymbolValue: ".org"},
						{Value: ".net", SymbolValue: ".net"},
						{Value: ".local", SymbolValue: ".ts.net"},
					},
					DisplayName:    hostname,
					KeyboardPrompt: hostname,
					Value:          hostname,
				},
			},
		},
		{
			Item: gabagool.MenuItem{Text: localize("login_port", "Port (optional)")},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					KeyboardLayout: gabagool.KeyboardLayoutNumeric,
					DisplayName:    port,
					KeyboardPrompt: port,
					Value:          port,
				},
			},
		},
		{
			Item: gabagool.MenuItem{Text: localize("login_ssl_certificates", "SSL Certificates")},
			Options: []gabagool.Option{
				{DisplayName: localize("login_ssl_verify", "Verify"), Value: false},
				{DisplayName: localize("login_ssl_skip", "Skip Verification"), Value: true},
			},
			SelectedOption: boolToIndex(host.InsecureSkipVerify),
			VisibleWhen:    sslVisible,
		},
	}

	result, err := gabagool.OptionsList(f.Title, gabagool.OptionListSettings{
		FooterHelpItems: f.Footer,
		StatusBar:       f.StatusBar,
		UseSmallTitle:   f.SmallTitle,
	}, items)
	if err != nil {
		if errors.Is(err, gabagool.ErrCancelled) {
			return host, false, nil
		}
		return host, false, err
	}

	// Hidden rows are still returned, so the SSL row can be read whether or not
	// the protocol left it on screen.
	fields := result.Items

	updated := host
	updated.RootURI = fields[0].Value().(string) + fields[1].Value().(string)
	updated.Port, _ = strconv.Atoi(fields[2].Value().(string))
	updated.InsecureSkipVerify = fields[3].Options[fields[3].SelectedOption].Value.(bool)

	return updated, true, nil
}

// removeScheme strips the protocol so the hostname field shows only what the
// user typed into it.
func removeScheme(rawURL string) string {
	if after, ok := strings.CutPrefix(rawURL, "https://"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(rawURL, "http://"); ok {
		return after
	}
	return rawURL
}
