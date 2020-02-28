package lccc_core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_log"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_strings"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/server"
	"github.com/micro/go-plugins/registry/etcdv3/v2"
	"go.etcd.io/etcd/clientv3"
	"strings"
	"sync"
	"time"
)

// 系统服务
type CoreServiceStruct struct {
	InstanceKey string `json:"instance_key"` // 服务实例的唯一键，系统运行时随机生成
	// 服务注册相关
	CoreStartParams *CoreStartParams                `json:"core_start_params"`     // 核心启动时传入的参数
	ServiceInstance *lccc_model.ServiceInstanceInfo `json:"self_service_instance"` // 服务自身的运行实例信息，根据配置和各种运行时信息，内部生成
	// 运行时缓存
	AllServiceInstancePoolCache map[string]map[string]*lccc_model.ServiceInstanceInfo `json:"all_service_instance_pool_cache"` // 所有服务实例信息池缓存，监听到服务实例有变动时刷新
	AllServiceBaseInfoPoolCache map[string]*lccc_model.ServiceBaseInfo
}

var coreServiceInstance *CoreServiceStruct
var coreServiceOnce sync.Once

// 单例函数
func CoreService() *CoreServiceStruct {
	coreServiceOnce.Do(func() {
		coreServiceInstance = &CoreServiceStruct{}
		coreServiceInstance.InstanceKey = lccu_strings.RandomUUIDString()
		coreServiceInstance.ServiceInstance = &lccc_model.ServiceInstanceInfo{}
	})
	return coreServiceInstance
}

// 注册微服务的时候需要传进来的配置信息
type CoreStartParams struct {
	RunGrpcService                 bool                               `json:"run_grpc_service"`         // 是否启用grpc服务，如网关调用时传false即可
	ServiceGeneralConfig           *lccc_model.GeneralConfig          `json:"service_general_config"`   // 通用配置，通常服务从本地配置文件中读取
	ServiceBaseInfo                *lccc_model.ServiceBaseInfo        `json:"service_base_info"`        // 服务的基础信息
	ServiceApplicationInfo         *lccc_model.ServiceApplicationInfo `json:"service_application_info"` // 服务可执行程序的应用信息，如版本数据等
	GrpcServiceImplRegisterHandler func(server server.Server)         // Grpc服务impl注册函数
	SystemSettingsDefine           *lccc_model.SystemSettingsDefine   `json:"system_settings_define"` // 服务所需系统设置定义
}

func (cs *CoreServiceStruct) GenerateMicroRegistry(config *lccc_model.GeneralConfig) registry.Registry {
	return etcdv3.NewRegistry(func(op *registry.Options) {
		op.Addrs = config.Registry.Endpoints
	})
}

// 供各种服务调用，启动时候调用此函数来在注册中心注册服务实例
func (cs *CoreServiceStruct) Start(params *CoreStartParams) error {
	// 在内存中存储和生成相关参数
	cs.CoreStartParams = params
	cs.ServiceInstance.EndpointHost = params.ServiceGeneralConfig.Service.OverrideHost
	cs.ServiceInstance.EndpointPort = params.ServiceGeneralConfig.Service.Port
	cs.ServiceInstance.ApplicationInfo = params.ServiceApplicationInfo
	// 初始化与注册中心ETCD的连接
	err := lccc_general_manager.EtcdManagerInstance().ClientInit(
		params.ServiceGeneralConfig.Registry.Endpoints,
		params.ServiceGeneralConfig.Registry.Username,
		params.ServiceGeneralConfig.Registry.Password)
	if err != nil {
		return err
	}
	// 初始化系统设置服务
	if err := SystemSettingsService().Init(); err != nil {
		lccu_log.Error("An error occurred while initializing the system settings service")
		return err
	}
	// 初始化数据沙箱服务
	if err := DataSandboxService().Init(); err != nil {
		lccu_log.Error("An error occurred while initializing the data sandbox service")
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
	// 注册微服务
	if params.RunGrpcService {
		service := micro.NewService(
			micro.Address(fmt.Sprintf(":%d", params.ServiceGeneralConfig.Service.Port)),
			micro.Registry(cs.GenerateMicroRegistry(params.ServiceGeneralConfig)),
			micro.Name(fmt.Sprintf("%v.%v", params.ServiceGeneralConfig.Service.Namespace, params.ServiceBaseInfo.ServiceKey)))
		service.Init()
		service.Server()
		if err := service.Run(); err != nil {
			return err
		}
	}
	return nil
}

func GetDefaultRegistryContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}

// 写入部分

// [instance]将自己的服务实例注册到注册中心服务实例列表中
func (cs *CoreServiceStruct) RegisterInstance() error {
	selfServiceKey := fmt.Sprintf("/%v/%v/%v/%v", KEY_C_SERVICE_INSTANCE, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace, cs.CoreStartParams.ServiceBaseInfo.ServiceKey, cs.InstanceKey)
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
func (cs *CoreServiceStruct) RegisterSystemSettingsDefine() error {
	key := fmt.Sprintf("/%v/%v/%v", KEY_C_SYSTEM_SETTINGS_DEFINE, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace, cs.CoreStartParams.ServiceBaseInfo.ServiceKey)
	jsonStr, err := json.Marshal(cs.CoreStartParams.SystemSettingsDefine)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), key, fmt.Sprintf("%s", jsonStr)); err != nil {
		return err
	}
	return nil
}

func (cs *CoreServiceStruct) RegisterServiceBaseInfo() error {
	key := fmt.Sprintf("/%v/%v/%v", KEY_C_SERVICE_BASE_INFO, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace, cs.CoreStartParams.ServiceBaseInfo.ServiceKey)
	jsonStr, err := json.Marshal(cs.CoreStartParams.ServiceBaseInfo)
	if err != nil {
		return err
	}
	if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), key, fmt.Sprintf("%s", jsonStr)); err != nil {
		return err
	}
	return nil
}

// 开始观察所有注册中心中的服务的变动，如果有变动，及时刷新
func (cs *CoreServiceStruct) StartWatchingAllService() {
	watchKey := fmt.Sprintf("/%v/%v", KEY_C_SERVICE_INSTANCE, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace)
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
func (cs *CoreServiceStruct) FastGetAllServiceInstance() map[string]map[string]*lccc_model.ServiceInstanceInfo {
	return cs.AllServiceInstancePoolCache
}

// 获取注册中心中所有服务实例，无视缓存，直接从注册中心中重新获取最新数据
func (cs *CoreServiceStruct) GetAllServiceInstance() (map[string]map[string]*lccc_model.ServiceInstanceInfo, error) {
	lccu_log.Info("Start refresh service instance data directly from the registry...")
	prefixKey := fmt.Sprintf("/%v/%v", KEY_C_SERVICE_INSTANCE, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(GetDefaultRegistryContext(), prefixKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	instancePool := make(map[string]map[string]*lccc_model.ServiceInstanceInfo)
	for _, kvs := range rsp.Kvs {
		fullKey := fmt.Sprintf("%s", kvs.Key)
		fullKeyComponents := strings.Split(fullKey, "/")
		if len(fullKeyComponents) < 5 {
			lccu_log.Error("The configuration key is invalid, skip and delete it! The format should be: /service_instance/$namespace/$service_key/$instance_key, but current key is: %s", fullKey)
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
func (cs *CoreServiceStruct) FastGetAllServiceBaseInfo() map[string]*lccc_model.ServiceBaseInfo {
	return cs.AllServiceBaseInfoPoolCache
}

// 从注册中心中获取所有服务的基本信息，无视缓存，直接从注册中心中获取最中心的数据
func (cs *CoreServiceStruct) GetAllServiceBaseInfo() (map[string]*lccc_model.ServiceBaseInfo, error) {
	lccu_log.Info("Start refresh service base info data directly from the registry...")
	prefixKey := fmt.Sprintf("/%v/%v", KEY_C_SERVICE_BASE_INFO, cs.CoreStartParams.ServiceGeneralConfig.Service.Namespace)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(GetDefaultRegistryContext(), prefixKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	baseInfoPool := make(map[string]*lccc_model.ServiceBaseInfo)
	for _, kvs := range rsp.Kvs {
		fullKey := fmt.Sprintf("%s", kvs.Key)
		fullKeyComponents := strings.Split(fullKey, "/")
		if len(fullKeyComponents) < 4 {
			lccu_log.Error("The configuration key is invalid, skip! The format should be: /service_base_info/$namespace/$service_key, but current key is: %s", fullKey)
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
