package lccc_core

import (
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_log"
	"github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/source/etcd"
	"sync"
)

type DataSandboxServiceStruct struct {
	DataSandboxContext config.Config
}

var dataSandboxServiceInstance *DataSandboxServiceStruct
var dataSandboxServiceOnce sync.Once

// 单例函数
func DataSandboxService() *DataSandboxServiceStruct {
	dataSandboxServiceOnce.Do(func() {
		dataSandboxServiceInstance = &DataSandboxServiceStruct{}
	})
	return dataSandboxServiceInstance
}

func (dss *DataSandboxServiceStruct) Init() error {
	var err error
	if dss.DataSandboxContext, err = config.NewConfig(); err != nil {
		return err
	}
	etcdSource := etcd.NewSource(
		etcd.WithAddress(CoreService().CoreStartParams.ServiceGeneralConfig.GetRegistryUrl()),
		etcd.WithPrefix(fmt.Sprintf("/%v/%v/%v", KEY_C_DATA_SANDBOX, CoreService().CoreStartParams.ServiceGeneralConfig.Service.Namespace, CoreService().CoreStartParams.ServiceBaseInfo.ServiceKey)),
		etcd.StripPrefix(true))
	if err = dss.DataSandboxContext.Load(etcdSource); err != nil {
		return err
	}
	return nil
}

func (dss *DataSandboxServiceStruct) Get(dataKey string) string {
	return dss.DataSandboxContext.Get(dataKey).String("")
}

func (dss *DataSandboxServiceStruct) Set(dataKey, newValue string) {
	dss.DataSandboxContext.Set(newValue, dataKey)
}

func (dss *DataSandboxServiceStruct) Watch(dataKey string, callback func(string)) {
	watch, err := dss.DataSandboxContext.Watch(dataKey)
	lccu_log.Error("An error occurred while observing the system settings：", err)
	go func() {
		if val, err := watch.Next(); err == nil {
			callback(val.String(""))
		} else {
			lccu_log.Error("An error occurred while observing the change of system settings：", err)
		}
		dss.Watch(dataKey, callback)
	}()
}
