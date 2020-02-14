package lccc_model

type SystemSettingItemDefine struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	UiType       string `json:"ui_type"`
	UiParams     string `json:"ui_params"`
	DefaultValue string `json:"default_value"`
	Introduce    string `json:"introduce"`
	NeedRestart  bool   `json:"need_restart"`
}

type SystemSettingGroupDefine struct {
	Key         string                     `json:"key"`
	Name        string                     `json:"name"`
	Introduce   string                     `json:"introduce"`
	SettingList []*SystemSettingItemDefine `json:"setting_list"`
}

type SystemSettingsDefine struct {
	Introduce        string                      `json:"introduce"`
	SettingGroupList []*SystemSettingGroupDefine `json:"setting_group_list"`
}
