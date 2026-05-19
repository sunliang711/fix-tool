package config

const (
	EnvPrefix         = "FIX_TOOL"
	DefaultConfigFile = "config/default.toml"
	UserConfigFile    = "config.toml"
	PrivateConfigFile = "private.toml"
)

type AppConfig struct {
	App     AppSettings   `mapstructure:"app" validate:"required"`
	Log     LogConfig     `mapstructure:"log" validate:"required"`
	Profile ProfileConfig `mapstructure:"profile" validate:"required"`
	Output  OutputConfig  `mapstructure:"output" validate:"required"`
}

type AppSettings struct {
	Name string `mapstructure:"name" validate:"required"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"required,oneof=console json"`
}

type ProfileConfig struct {
	Name                    string            `mapstructure:"name" validate:"required"`
	BeginString             string            `mapstructure:"begin_string" validate:"required"`
	SenderCompID            string            `mapstructure:"sender_comp_id" validate:"required"`
	TargetCompID            string            `mapstructure:"target_comp_id" validate:"required"`
	Username                string            `mapstructure:"username"`
	Password                string            `mapstructure:"password"`
	Host                    string            `mapstructure:"host" validate:"required"`
	Port                    int               `mapstructure:"port" validate:"min=1,max=65535"`
	TLS                     TLSConfig         `mapstructure:"tls" validate:"required"`
	HeartbeatInterval       string            `mapstructure:"heartbeat_interval" validate:"required"`
	ResetOnLogon            bool              `mapstructure:"reset_on_logon"`
	DataDictionary          string            `mapstructure:"data_dictionary"`
	TransportDataDictionary string            `mapstructure:"transport_data_dictionary"`
	AppDataDictionary       string            `mapstructure:"app_data_dictionary"`
	CustomTags              []CustomTagConfig `mapstructure:"custom_tags"`
}

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
}

type CustomTagConfig struct {
	Tag         int               `mapstructure:"tag" validate:"min=1"`
	Name        string            `mapstructure:"name" validate:"required"`
	Type        string            `mapstructure:"type" validate:"required"`
	Required    bool              `mapstructure:"required"`
	Sensitive   bool              `mapstructure:"sensitive"`
	Enums       map[string]string `mapstructure:"enums"`
	Description string            `mapstructure:"description"`
}

type OutputConfig struct {
	Format          string `mapstructure:"format" validate:"required,oneof=table raw json"`
	RawDelimiter    string `mapstructure:"raw_delimiter" validate:"required"`
	RedactSensitive bool   `mapstructure:"redact_sensitive"`
}
