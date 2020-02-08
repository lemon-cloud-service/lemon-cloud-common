package config

type GeneralConfig struct {
	Registry struct {
		RegistryType string
		Endpoints    []string
		Username     string
		Password     string
	}
}
