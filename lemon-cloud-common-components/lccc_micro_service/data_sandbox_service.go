package lccc_micro_service

import (
	"sync"
)

type DataSandboxService struct {
}

var dataSandboxServiceInstance *DataSandboxService
var dataSandboxServiceOnce sync.Once

// 单例函数
func DataSandboxServiceSingletonInstance() *DataSandboxService {
	dataSandboxServiceOnce.Do(func() {
		dataSandboxServiceInstance = &DataSandboxService{}
	})
	return dataSandboxServiceInstance
}
