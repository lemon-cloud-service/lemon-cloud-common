package lccc_model

type GeneralConfig struct {
	Registry struct {
		Kind      string
		Endpoints []string
		Username  string
		Password  string
	}
	Service struct {
		Namespace    string
		SecretId     string `yaml:"secret_id"`
		SecretKey    string `yaml:"secret_key"`
		OverrideHost string `yaml:"override_host"`
		Port         uint16
	}
}
