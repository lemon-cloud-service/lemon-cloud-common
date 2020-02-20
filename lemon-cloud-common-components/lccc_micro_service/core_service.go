package lccc_micro_service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_log"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_strings"
	"go.etcd.io/etcd/clientv3"
	"strings"
	"sync"
	"time"
)

// 系统服务
type CoreService struct {
	InstanceKey string `json:"instance_key"` // 服务实例的唯一键，系统运行时随机生成
	// 服务注册相关
	ServiceGeneralConfig   *lccc_model.GeneralConfig          `json:"service_general_config"`   // 服务的通用配置，注册时传入
	ServiceBaseInfo        *lccc_model.ServiceBaseInfo        `json:"service_base_info"`        // 服务的基本信息，注册时传入
	ServiceApplicationInfo *lccc_model.ServiceApplicationInfo `json:"service_application_info"` // 服务的可执行程序的应用信息，注册时传入
	ServiceInstance        *lccc_model.ServiceInstanceInfo    `json:"self_service_instance"`    // 服务自身的运行实例信息，根据配置和各种运行时信息，内部生成
	// 系统设置相关
	SystemSettingsDefine *lccc_model.SystemSettingsDefine `json:"system_settings_define"` // 服务所需的系统设置项的定义，注册时传入
	// 运行时缓存
	AllServiceInstancePoolCache map[string]map[string]*lccc_model.ServiceInstanceInfo `json:"all_service_instance_pool_cache"`  // 所有服务实例信息池缓存，监听到服务实例有变动时刷新
	AllServiceBaseInfoPoolCache map[string]*lccc_model.ServiceBaseInfo                `json:"all_service_base_info_pool_cache"` // 所有服务基本信息池缓存，监听到服务实例有变动时刷新
}

var coreServiceInstance *CoreService
var coreServiceOnce sync.Once

// 单例函数
func CoreServiceSingletonInstance() *CoreService {
	coreServiceOnce.Do(func() {
		coreServiceInstance = &CoreService{}
		coreServiceInstance.InstanceKey = lccu_strings.RandomUUIDString()
		coreServiceInstance.ServiceInstance = &lccc_model.ServiceInstanceInfo{}
	})
	return coreServiceInstance
}

// 注册微服务的时候需要传进来的配置信息
type ServiceRegisterConfig struct {
	ServiceGeneralConfig   *lccc_model.GeneralConfig          `json:"service_general_config"`   // 通用配置，通常服务从本地配置文件中读取
	ServiceBaseInfo        *lccc_model.ServiceBaseInfo        `json:"service_base_info"`        // 服务的基础信息
	ServiceApplicationInfo *lccc_model.ServiceApplicationInfo `json:"service_application_info"` // 服务可执行程序的应用信息，如版本数据等
}

// 供各种服务调用，启动时候调用此函数来在注册中心注册服务实例
func (cs *CoreService) RegisterServiceInstance(
	config *ServiceRegisterConfig,
	systemSettingsDefine *lccc_model.SystemSettingsDefine) error {
	// 在内存中存储和生成相关参数
	cs.ServiceGeneralConfig = config.ServiceGeneralConfig
	cs.ServiceBaseInfo = config.ServiceBaseInfo
	cs.ServiceApplicationInfo = config.ServiceApplicationInfo
	cs.ServiceInstance.EndpointHost = cs.ServiceGeneralConfig.Service.OverrideHost
	cs.ServiceInstance.EndpointPort = cs.ServiceGeneralConfig.Service.Port
	cs.ServiceInstance.ApplicationInfo = config.ServiceApplicationInfo
	cs.SystemSettingsDefine = systemSettingsDefine
	// 初始化与注册中心ETCD的连接
	err := lccc_general_manager.EtcdManagerInstance().ClientInit(
		config.ServiceGeneralConfig.Registry.Endpoints,
		config.ServiceGeneralConfig.Registry.Username,
		config.ServiceGeneralConfig.Registry.Password)
	if err != nil {
		return err
	}
	// 监视注册中心中的服务实例列表的变动
	cs.StartWatchingAllService()
	// 向注册中心写入 - 服务实例信息 instance
	if err = cs.RegisterInstance(); err != nil {
		return err
	}
	// 向注册中心写入 - 服务基本信息 service_base_info
	if err = cs.RegisterServiceBaseInfo(); err != nil {
		return err
	}
	// 向注册中心写入 - 系统设置定义 system_settings_define
	if err = cs.RegisterSystemSettingsDefine(); err != nil {
		return err
	}
	return nil
}

// 写入部分

// [instance]将自己的服务实例注册到注册中心服务实例列表中
func (cs *CoreService) RegisterInstance() error {
	selfServiceKey := fmt.Sprintf("%v.%v.%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SERVICE_INSTANCE, cs.ServiceBaseInfo.ServiceKey, cs.InstanceKey)
	lease := clientv3.NewLease(lccc_general_manager.EtcdManagerInstance().ClientInstance())
	var leaseGrantResp *clientv3.LeaseGrantResponse
	var err error
	if leaseGrantResp, err = lease.Grant(GetDefaultRegistryContext(), 10); err != nil {
		return err
	}
	leaseId := leaseGrantResp.ID
	var keepRespChan <-chan *clientv3.LeaseKeepAliveResponse
	if keepRespChan, err = lease.KeepAlive(context.Background(), leaseId); err != nil {
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
					lccu_log.Info("Successful keep registry instance: ", selfServiceKey, " , LEASE ID = ", keepResp.ID)
				}
			}
		}
	END:
		lccu_log.Error("Disconnected to registry, try to reconnect to the registry")
		_ = cs.RegisterInstance()
	}()
	// 将服务实例信息序列化成JSON并写入到注册中心实例列表中
	jsonData, err := json.Marshal(cs.ServiceInstance)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(
		context.Background(), selfServiceKey, fmt.Sprintf("%s", jsonData), clientv3.WithLease(leaseId)); err != nil {
		return err
	}
	return nil
}

// [system_settings_define]向注册中心中写入本系统的系统设置定义数据
func (cs *CoreService) RegisterSystemSettingsDefine() error {
	key := fmt.Sprintf("%v.%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SYSTEM_SETTINGS_DEFINE, cs.ServiceBaseInfo.ServiceKey)
	json, err := json.Marshal(cs.SystemSettingsDefine)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(GetDefaultRegistryContext(), key, fmt.Sprintf("%s", json)); err != nil {
		return err
	}
	return nil
}

func (cs *CoreService) RegisterServiceBaseInfo() error {
	key := fmt.Sprintf("%v.%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SERVICE_BASE_INFO, cs.ServiceBaseInfo.ServiceKey)
	json, err := json.Marshal(cs.ServiceBaseInfo)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(GetDefaultRegistryContext(), key, fmt.Sprintf("%s", json)); err != nil {
		return err
	}
	return nil
}

// 开始观察所有注册中心中的服务的变动，如果有变动，及时刷新
func (cs *CoreService) StartWatchingAllService() {
	watchKey := fmt.Sprintf("%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SERVICE_INSTANCE)
	watchChain := lccc_general_manager.EtcdManagerInstance().ClientInstance().Watch(context.Background(), watchKey, clientv3.WithPrefix())
	go func() {
		for range watchChain {
			var err error = nil
			_, err = cs.GetAllServiceInstance()
			if err != nil {
				lccu_log.Error("Observed a change in the service instance of the registry, but failed to refresh the instance list, reason: ", err.Error())
			}
			_, err = cs.GetAllServiceBaseInfo()
			if err != nil {
				lccu_log.Error("Observed a change in the service base info of the registry, but failed to refresh the instance list, reason: ", err.Error())
			}
		}
	}()
}

// 快速获取所有服务的实例池（从缓存中获取）
func (cs *CoreService) FastGetAllServiceInstance() map[string]map[string]*lccc_model.ServiceInstanceInfo {
	return cs.AllServiceInstancePoolCache
}

// 获取注册中心中所有服务实例，无视缓存，直接从注册中心中重新获取最新数据
func (cs *CoreService) GetAllServiceInstance() (map[string]map[string]*lccc_model.ServiceInstanceInfo, error) {
	lccu_log.Info("Start refresh service instance data directly from the registry...")
	prefixKey := fmt.Sprintf("%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SERVICE_INSTANCE)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(GetDefaultRegistryContext(), prefixKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	instancePool := make(map[string]map[string]*lccc_model.ServiceInstanceInfo)
	for _, kvs := range rsp.Kvs {
		fullKey := fmt.Sprintf("%s", kvs.Key)
		fullKeyComponents := strings.Split(fullKey, ".")
		if len(fullKeyComponents) < 4 {
			lccu_log.Error("The configuration key is invalid, skip and delete it! The format should be: $namespace.service_instance.$service_key.$instance_key, but current key is: %s", fullKey)
			if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Delete(GetDefaultRegistryContext(), fullKey); err != nil {
				lccu_log.Error("Invalid instance key deletion failed, reason: %v", err.Error())
			}
			continue
		}
		serviceKey := fullKeyComponents[len(fullKeyComponents)-2]
		if _, ok := instancePool[serviceKey]; !ok {
			// serviceKey 不存在，创建一个serviceKey对应的map
			instancePool[serviceKey] = make(map[string]*lccc_model.ServiceInstanceInfo)
		}
		instance := &lccc_model.ServiceInstanceInfo{}
		if err := json.Unmarshal(kvs.Value, instance); err != nil {
			lccu_log.Error("Parse instance JSON registration data failed, skip parsing the data and delete it, JSON data: \n %s", kvs.Value)
			if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Delete(GetDefaultRegistryContext(), fullKey); err != nil {
				lccu_log.Error("Invalid instance JSON instance registration data deletion failed, reason: %v", err.Error())
			}
			continue
		}
		instanceKey := fullKeyComponents[len(fullKeyComponents)-1]
		instancePool[serviceKey][instanceKey] = instance
	}
	// save to cache
	cs.AllServiceInstancePoolCache = instancePool
	lccu_log.Info("Refreshing service instance data succeeded")
	return instancePool, nil
}

// 快速获取所有服务的基础信息池（从缓存中获取）
func (cs *CoreService) FastGetAllServiceBaseInfo() map[string]*lccc_model.ServiceBaseInfo {
	return cs.AllServiceBaseInfoPoolCache
}

// 从注册中心中获取所有服务的基本信息，无视缓存，直接从注册中心中获取最中心的数据
func (cs *CoreService) GetAllServiceBaseInfo() (map[string]*lccc_model.ServiceBaseInfo, error) {
	lccu_log.Info("Start refresh service base info data directly from the registry...")
	prefixKey := fmt.Sprintf("%v.%v", cs.ServiceGeneralConfig.Service.Namespace, KEY_C_SERVICE_BASE_INFO)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(GetDefaultRegistryContext(), prefixKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	baseInfoPool := make(map[string]*lccc_model.ServiceBaseInfo)
	for _, kvs := range rsp.Kvs {
		fullKey := fmt.Sprintf("%s", kvs.Key)
		fullKeyComponents := strings.Split(fullKey, ".")
		if len(fullKeyComponents) < 3 {
			lccu_log.Error("The configuration key is invalid, skip! The format should be: $namespace.service_base_info.$service_key, but current key is: %s", fullKey)
			if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Delete(GetDefaultRegistryContext(), fullKey); err != nil {
				lccu_log.Error("Invalid base info key deletion failed, reason: %v", err.Error())
			}
			continue
		}
		baseInfo := &lccc_model.ServiceBaseInfo{}
		if err := json.Unmarshal(kvs.Value, baseInfo); err != nil {
			lccu_log.Error("Parse base_info JSON registration data failed, skip parsing the data and delete it, JSON data: \n %s", kvs.Value)
			if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Delete(GetDefaultRegistryContext(), fullKey); err != nil {
				lccu_log.Error("Invalid instance JSON base_info registration data deletion failed, reason: %v", err.Error())
			}
			continue
		}
		serviceKey := fullKeyComponents[len(fullKeyComponents)-1]
		baseInfoPool[serviceKey] = baseInfo
	}
	// save to cache
	cs.AllServiceBaseInfoPoolCache = baseInfoPool
	lccu_log.Info("Refreshing service base info data succeeded")
	return baseInfoPool, nil
}

func GetDefaultRegistryContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}
