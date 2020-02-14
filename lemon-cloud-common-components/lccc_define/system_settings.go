package lccc_define

import "github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"

var GENERAL_SYSTEM_SETTING_ITEM_DEFINE_DATABASE_TYPE = &lccc_model.SystemSettingItemDefine{
	Key:          "database_type",
	Name:         "数据库类型",
	UiType:       "select",
	UiParams:     `{"min":1, "max": 1", "options": [{"value": "mysql", "text": "MySQL"}, {"value": "postgres", "text": "PostgreSQL"}, {"value": "sqlite", "text": "Sqlite3"}, {"value": "mssql", "text": "SQL Server"}]}`,
	DefaultValue: "mysql",
	Introduce:    "要连接的数据库的类型",
	NeedRestart:  true,
}
var GENERAL_SYSTEM_SETTING_ITEM_DEFINE_DATABASE_URL = &lccc_model.SystemSettingItemDefine{
	Key:          "database_url",
	Name:         "数据库连接URL",
	UiType:       "input",
	UiParams:     "",
	DefaultValue: "",
	Introduce:    "数据库连接URL，Sqlite3数据库为本地数据库文件地址",
	NeedRestart:  true,
}
