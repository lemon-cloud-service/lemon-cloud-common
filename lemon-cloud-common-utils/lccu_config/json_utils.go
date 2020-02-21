package lccu_config

import (
	"encoding/json"
	"io/ioutil"
)

func LoadJsonConfigFile(filePath string, out interface{}) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
