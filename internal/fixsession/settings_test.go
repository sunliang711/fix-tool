package fixsession

import (
	"strings"
	"testing"

	"fix-tool/internal/config"

	qfconfig "github.com/quickfixgo/quickfix/config"
)

func TestSettingsFromProfileMapsQuickFIXSettings(t *testing.T) {
	profile := config.ProfileConfig{
		Name:                    "uat",
		BeginString:             "FIX.4.4",
		SenderCompID:            "BUY",
		TargetCompID:            "SELL",
		Host:                    "fix.example.test",
		Port:                    5001,
		HeartbeatInterval:       "45s",
		ResetOnLogon:            true,
		DataDictionary:          "FIX44.xml",
		TransportDataDictionary: "FIXT11.xml",
		AppDataDictionary:       "FIX50SP2.xml",
		TLS: config.TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: false,
			CAFile:             "ca.pem",
			CertFile:           "client.pem",
			KeyFile:            "client.key",
		},
	}

	settings, sessionID, err := SettingsFromProfile(profile)
	if err != nil {
		t.Fatalf("SettingsFromProfile() error = %v", err)
	}

	if sessionID.BeginString != profile.BeginString {
		t.Fatalf("BeginString = %q, want %q", sessionID.BeginString, profile.BeginString)
	}
	if sessionID.SenderCompID != profile.SenderCompID {
		t.Fatalf("SenderCompID = %q, want %q", sessionID.SenderCompID, profile.SenderCompID)
	}
	if sessionID.TargetCompID != profile.TargetCompID {
		t.Fatalf("TargetCompID = %q, want %q", sessionID.TargetCompID, profile.TargetCompID)
	}

	sessionSettings := settings.SessionSettings()[sessionID]
	assertSetting(t, sessionSettings, qfconfig.SocketConnectHost, "fix.example.test")
	assertSetting(t, sessionSettings, qfconfig.SocketConnectPort, "5001")
	assertSetting(t, sessionSettings, qfconfig.HeartBtInt, "45")
	assertSetting(t, sessionSettings, qfconfig.ResetOnLogon, "Y")
	assertSetting(t, sessionSettings, qfconfig.DataDictionary, "FIX44.xml")
	assertSetting(t, sessionSettings, qfconfig.TransportDataDictionary, "FIXT11.xml")
	assertSetting(t, sessionSettings, qfconfig.AppDataDictionary, "FIX50SP2.xml")
	assertSetting(t, sessionSettings, qfconfig.SocketUseSSL, "Y")
	assertSetting(t, sessionSettings, qfconfig.SocketInsecureSkipVerify, "N")
	assertSetting(t, sessionSettings, qfconfig.SocketCAFile, "ca.pem")
	assertSetting(t, sessionSettings, qfconfig.SocketCertificateFile, "client.pem")
	assertSetting(t, sessionSettings, qfconfig.SocketPrivateKeyFile, "client.key")
}

func TestSettingsFromProfileMapsInsecureSkipVerify(t *testing.T) {
	profile := validProfile()
	profile.TLS.Enabled = true
	profile.TLS.InsecureSkipVerify = true

	settings, sessionID, err := SettingsFromProfile(profile)
	if err != nil {
		t.Fatalf("SettingsFromProfile() error = %v", err)
	}

	sessionSettings := settings.SessionSettings()[sessionID]
	assertSetting(t, sessionSettings, qfconfig.SocketUseSSL, "Y")
	assertSetting(t, sessionSettings, qfconfig.SocketInsecureSkipVerify, "Y")
}

func TestSettingsFromProfileRejectsSubSecondHeartbeat(t *testing.T) {
	profile := validProfile()
	profile.HeartbeatInterval = "500ms"

	_, _, err := SettingsFromProfile(profile)
	if err == nil {
		t.Fatal("SettingsFromProfile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "at least one second") {
		t.Fatalf("SettingsFromProfile() error = %v, want heartbeat error", err)
	}
}

func assertSetting(t *testing.T, settings settingReader, key string, want string) {
	t.Helper()
	got, err := settings.Setting(key)
	if err != nil {
		t.Fatalf("Setting(%s) error = %v", key, err)
	}
	if got != want {
		t.Fatalf("Setting(%s) = %q, want %q", key, got, want)
	}
}

type settingReader interface {
	Setting(setting string) (string, error)
}

func validProfile() config.ProfileConfig {
	return config.ProfileConfig{
		Name:              "default",
		BeginString:       "FIX.4.4",
		SenderCompID:      "SENDER",
		TargetCompID:      "TARGET",
		Host:              "127.0.0.1",
		Port:              9876,
		HeartbeatInterval: "30s",
		TLS: config.TLSConfig{
			Enabled: true,
		},
	}
}
