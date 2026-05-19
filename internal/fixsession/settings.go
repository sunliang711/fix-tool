package fixsession

import (
	"fmt"
	"strconv"
	"time"

	"fix-tool/internal/config"

	"github.com/quickfixgo/quickfix"
	qfconfig "github.com/quickfixgo/quickfix/config"
)

const (
	settingConnectionType           = "ConnectionType"
	settingConnectionTypeInitiator  = "initiator"
	defaultReconnectIntervalSeconds = 30
)

func SettingsFromProfile(profile config.ProfileConfig) (*quickfix.Settings, quickfix.SessionID, error) {
	settings := quickfix.NewSettings()
	settings.GlobalSettings().Set(settingConnectionType, settingConnectionTypeInitiator)
	settings.GlobalSettings().Set(qfconfig.ReconnectInterval, strconv.Itoa(defaultReconnectIntervalSeconds))

	sessionSettings := quickfix.NewSessionSettings()
	sessionSettings.Set(qfconfig.BeginString, profile.BeginString)
	sessionSettings.Set(qfconfig.SenderCompID, profile.SenderCompID)
	sessionSettings.Set(qfconfig.TargetCompID, profile.TargetCompID)
	sessionSettings.Set(qfconfig.SocketConnectHost, profile.Host)
	sessionSettings.Set(qfconfig.SocketConnectPort, strconv.Itoa(profile.Port))
	sessionSettings.Set(qfconfig.ResetOnLogon, boolToFIX(profile.ResetOnLogon))

	heartBtInt, err := heartBtIntSeconds(profile.HeartbeatInterval)
	if err != nil {
		return nil, quickfix.SessionID{}, err
	}
	sessionSettings.Set(qfconfig.HeartBtInt, strconv.Itoa(heartBtInt))

	if profile.DataDictionary != "" {
		sessionSettings.Set(qfconfig.DataDictionary, profile.DataDictionary)
	}
	if profile.TransportDataDictionary != "" {
		sessionSettings.Set(qfconfig.TransportDataDictionary, profile.TransportDataDictionary)
	}
	if profile.AppDataDictionary != "" {
		sessionSettings.Set(qfconfig.AppDataDictionary, profile.AppDataDictionary)
	}

	applyTLSSettings(sessionSettings, profile.TLS)

	sessionID, err := settings.AddSession(sessionSettings)
	if err != nil {
		return nil, quickfix.SessionID{}, fmt.Errorf("add quickfix session settings: %w", err)
	}
	return settings, sessionID, nil
}

func applyTLSSettings(settings *quickfix.SessionSettings, tlsConfig config.TLSConfig) {
	settings.Set(qfconfig.SocketUseSSL, boolToFIX(tlsConfig.Enabled))
	if !tlsConfig.Enabled {
		return
	}
	settings.Set(qfconfig.SocketInsecureSkipVerify, boolToFIX(tlsConfig.InsecureSkipVerify))
	if tlsConfig.CAFile != "" {
		settings.Set(qfconfig.SocketCAFile, tlsConfig.CAFile)
	}
	if tlsConfig.CertFile != "" {
		settings.Set(qfconfig.SocketCertificateFile, tlsConfig.CertFile)
	}
	if tlsConfig.KeyFile != "" {
		settings.Set(qfconfig.SocketPrivateKeyFile, tlsConfig.KeyFile)
	}
}

func heartBtIntSeconds(value string) (int, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse heartbeat interval: %w", err)
	}
	if duration < time.Second {
		return 0, fmt.Errorf("heartbeat interval must be at least one second")
	}
	if duration%time.Second != 0 {
		return 0, fmt.Errorf("heartbeat interval must use whole seconds")
	}
	return int(duration / time.Second), nil
}

func boolToFIX(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}
