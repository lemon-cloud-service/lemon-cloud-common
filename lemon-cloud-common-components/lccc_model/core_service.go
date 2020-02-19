package lccc_model

// 用于描述某一服务的基本信息，如服务名称，服务介绍等
type ServiceBaseInfo struct {
	ServiceKey       string `json:"service_key"`       // 服务的唯一标识
	ServiceName      string `json:"service_name"`      // 服务名称
	ServiceIntroduce string `json:"service_introduce"` // 服务的介绍
}

// 用于描述某一服务的可执行应用的信息，如应用程序版本号等
type ServiceApplicationInfo struct {
	ApplicationVersion    string `json:"service_version"`     // 应用的可读版本号字符串
	ApplicationVersionNum uint16 `json:"service_version_num"` // 应用的版本号码
}

// 用于描述某服务的某一个运行实例信息
type ServiceInstanceInfo struct {
	EndpointHost    string                  `json:"endpoint_host"`    // 服务运行所在主机的访问地址
	EndpointPort    uint16                  `json:"endpoint_port"`    // 服务开放的端口号
	ApplicationInfo *ServiceApplicationInfo `json:"application_info"` // 服务实例所使用的的可执行程序的信息
}
