package lccu_config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func LoadYamlConfigFile(filePath string, out interface{}) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}
