package lccc_micro_service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_strings"
	"go.etcd.io/etcd/clientv3"
	"sync"
)

const KEY_C_COMPONENTS = "components"
const KEY_C_SETTINGS_DEFINE = "settings_define"
const KEY_C_SETTINGS_VALUE = "settings_value"

type SystemService struct {
	InstanceKey string
	// 服务注册相关
	ServiceGeneralConfig *lccc_model.GeneralConfig
	ServiceInfo          *lccc_model.ServiceInfo
	ServicePool          *ServicePool
	SelfServiceItem      *ServiceItem
	// 系统设置相关
	SystemSettingsDefine *lccc_model.SystemSettingsDefine
}

var systemServiceInstance *SystemService
var systemServiceOnce sync.Once

func SystemServiceInstance() *SystemService {
	systemServiceOnce.Do(func() {
		systemServiceInstance = &SystemService{}
		systemServiceInstance.InstanceKey = lccu_strings.RandomUUIDString()
		systemServiceInstance.SelfServiceItem = &ServiceItem{}
	})
	return systemServiceInstance
}

// Micro Service Register Config
type ServiceRegisterConfig struct {
	ServiceGeneralConfig *lccc_model.GeneralConfig
	ServiceInfo          *lccc_model.ServiceInfo
}

type ServiceItem struct {
	EndpointHost string `json:"endpoint_host"`
	EndpointPort uint16 `json:"endpoint_port"`
}

type ServicePool struct {
	ServerList map[string]*ServiceItem `json:"server_list"`
}

func (ss *SystemService) RegisterNewService(
	config *ServiceRegisterConfig,
	systemSettingsDefine *lccc_model.SystemSettingsDefine) error {
	// 存储生成相关参数
	ss.ServiceGeneralConfig = config.ServiceGeneralConfig
	ss.ServiceInfo = config.ServiceInfo
	ss.SelfServiceItem.EndpointHost = ss.ServiceGeneralConfig.Service.OverrideHost
	ss.SelfServiceItem.EndpointPort = ss.ServiceGeneralConfig.Service.Port
	ss.SystemSettingsDefine = systemSettingsDefine
	// 注册到注册中心
	err := lccc_general_manager.EtcdManagerInstance().ClientInit(
		config.ServiceGeneralConfig.Registry.Endpoints,
		config.ServiceGeneralConfig.Registry.Username,
		config.ServiceGeneralConfig.Registry.Password)
	if err != nil {
		return err
	}
	// 	监视注册中心中的服务
	ss.StartWatchingAllService()
	// register self to registry
	if err = ss.RegisterSelf(); err != nil {
		return err
	}
	// 向注册中心写入 - 系统设置定义
	if err = ss.WriteSystemSettingsDefineData(); err != nil {
		return err
	}
	return nil
}

// 将自己注册到注册中心服务列表中
func (ss *SystemService) RegisterSelf() error {
	selfServiceKey := fmt.Sprintf("%v.%v.%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_COMPONENTS, ss.ServiceInfo.ServiceTag, ss.InstanceKey)
	lease := clientv3.NewLease(lccc_general_manager.EtcdManagerInstance().ClientInstance())
	var leaseGrantResp *clientv3.LeaseGrantResponse
	var err error
	if leaseGrantResp, err = lease.Grant(context.Background(), 10); err != nil {
		return err
	}
	leaseId := leaseGrantResp.ID
	var keepRespChan <-chan *clientv3.LeaseKeepAliveResponse
	if keepRespChan, err = lease.KeepAlive(context.Background(), leaseId); err != nil {
		fmt.Println(err)
		return err
	}
	// listen lease chain
	go func() {
		for {
			select {
			case keepResp := <-keepRespChan:
				if keepRespChan == nil {
					goto END
				} else {
					fmt.Printf("Successful keep: %v , LEASE ID = %d \n", selfServiceKey, keepResp.ID)
				}
			}
		}
	END:
		fmt.Println("disconnected to registry")
		_ = ss.RegisterSelf()
	}()
	jsonData, err := json.Marshal(ss.SelfServiceItem)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(
		context.Background(), selfServiceKey, fmt.Sprintf("%s", jsonData), clientv3.WithLease(leaseId)); err != nil {
		return err
	}
	return nil
}

// 同步注册中心中的所有服务
func (ss *SystemService) SyncAllServiceInfo() error {
	watchKey := fmt.Sprintf("%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_COMPONENTS)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(context.Background(), watchKey, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kvs := range rsp.Kvs {
		fmt.Printf("key: %s, value: %s\n", kvs.Key, kvs.Value)
	}
	return nil
}

// 开始观察所有注册中心中的服务的变动，如果有变动，及时刷新
func (ss *SystemService) StartWatchingAllService() {
	watchKey := fmt.Sprintf("%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_COMPONENTS)
	watchChain := lccc_general_manager.EtcdManagerInstance().ClientInstance().Watch(context.Background(), watchKey, clientv3.WithPrefix())
	go func() {
		for range watchChain {
			_ = ss.SyncAllServiceInfo()
		}
	}()
}

// 向注册中心中写入本系统的系统设置定义数据
func (ss *SystemService) WriteSystemSettingsDefineData() error {
	key := fmt.Sprintf("%v.%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_SETTINGS_DEFINE, ss.ServiceInfo.ServiceTag)
	json, err := json.Marshal(ss.SystemSettingsDefine)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), key, fmt.Sprintf("%s", json)); err != nil {
		return err
	}
	return nil
}
