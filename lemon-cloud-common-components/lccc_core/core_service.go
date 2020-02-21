package lccc_core

import (
	"context"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/server"
	"github.com/micro/go-plugins/registry/etcdv3/v2"
	"sync"
	"time"
)

// 系统服务
type CoreServiceStruct struct {
	// 服务注册相关
	CoreStartParams *CoreStartParams `json:"core_start_params"` // 核心启动时传入的参数
}

var coreServiceInstance *CoreServiceStruct
var coreServiceOnce sync.Once

// 单例函数
func CoreService() *CoreServiceStruct {
	coreServiceOnce.Do(func() {
		coreServiceInstance = &CoreServiceStruct{}
	})
	return coreServiceInstance
}

// 注册微服务的时候需要传进来的配置信息
type CoreStartParams struct {
	ServiceGeneralConfig           *lccc_model.GeneralConfig          `json:"service_general_config"`   // 通用配置，通常服务从本地配置文件中读取
	ServiceBaseInfo                *lccc_model.ServiceBaseInfo        `json:"service_base_info"`        // 服务的基础信息
	ServiceApplicationInfo         *lccc_model.ServiceApplicationInfo `json:"service_application_info"` // 服务可执行程序的应用信息，如版本数据等
	GrpcServiceImplRegisterHandler func(server server.Server)         // Grpc服务impl注册函数
	SystemSettingsDefine           *lccc_model.SystemSettingsDefine   `json:"system_settings_define"` // 服务所需系统设置定义
}

// 供各种服务调用，启动时候调用此函数来在注册中心注册服务实例
func (cs *CoreServiceStruct) Start(params *CoreStartParams) error {
	// 在内存中存储和生成相关参数
	cs.CoreStartParams = params
	// 注册微服务
	etcdv3.NewRegistry()
	registry := etcdv3.NewRegistry(func(op *registry.Options) {
		op.Addrs = params.ServiceGeneralConfig.Registry.Endpoints
	})
	service := micro.NewService(
		micro.Registry(registry),
		micro.Name(fmt.Sprintf("%v.%v", params.ServiceGeneralConfig.Service.Namespace, params.ServiceBaseInfo.ServiceKey)))
	service.Init()
	service.Server()
	// run server
	if err := service.Run(); err != nil {
		return err
	}
	return nil
}

func GetDefaultRegistryContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}
