package lccc_micro_service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-model/lccm_config"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_strings"
	"go.etcd.io/etcd/clientv3"
	"sync"
)

const KEY_C_COMPONENTS = "components"
const KEY_C_SERVICE_POOL = "service_pool"

type SystemService struct {
	InstanceKey          string
	ServiceGeneralConfig *lccm_config.GeneralConfig
	ServiceInfo          *lccm_config.ServiceInfo
	ServicePool          *ServicePool
}

var systemServiceInstance *SystemService
var systemServiceOnce sync.Once

func SystemServiceInstance() *SystemService {
	systemServiceOnce.Do(func() {
		systemServiceInstance = &SystemService{}
		systemServiceInstance.InstanceKey = lccu_strings.RandomUUIDString()
	})
	return systemServiceInstance
}

// Micro Service Register Config
type ServiceRegisterConfig struct {
	ServiceGeneralConfig *lccm_config.GeneralConfig
	ServiceInfo          *lccm_config.ServiceInfo
}

type ServiceEndpoint struct {
	Host string `json:"host"`
	Port uint16 `json:"port"`
}

type ServiceItem struct {
	ServiceIntroduce string             `json:"service_introduce"`
	Endpoints        []*ServiceEndpoint `json:"endpoints"`
}

type ServicePool struct {
	ServerList map[string]*ServiceItem `json:"server_list"`
}

func (ss *SystemService) RegisterNewService(config *ServiceRegisterConfig) error {
	// save data
	ss.ServiceGeneralConfig = config.ServiceGeneralConfig
	ss.ServiceInfo = config.ServiceInfo
	// connect to registry
	err := lccc_general_manager.EtcdManagerInstance().ClientInit(
		config.ServiceGeneralConfig.Registry.Endpoints,
		config.ServiceGeneralConfig.Registry.Username,
		config.ServiceGeneralConfig.Registry.Password)
	if err != nil {
		return err
	}

	watchKey := fmt.Sprintf("%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_COMPONENTS)
	fmt.Println(watchKey)
	//lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), watchKey, "kugou")
	watchChain := lccc_general_manager.EtcdManagerInstance().ClientInstance().Watch(context.Background(), watchKey, clientv3.WithPrefix())
	go func() {
		for res := range watchChain {
			value := res.Events[0].Kv.Value
			for _, event := range res.Events {
				fmt.Printf("!!!! watch change!!!   key: %s, value: %s\n", event.Kv.Key, event.Kv.Value)
			}
			fmt.Printf("??? : %s\n", value)
		}
	}()
	putRsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), fmt.Sprintf("%v.%v.%v", watchKey, ss.ServiceInfo.ServiceTag, ss.InstanceKey), "uuuuuser")
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(context.Background(), watchKey, clientv3.WithPrefix())
	for _, kvs := range rsp.Kvs {
		fmt.Printf("key: %s, value: %s\n", kvs.Key, kvs.Value)
	}
	fmt.Println(rsp)
	// set micro service info into registry
	//err = ss.SyncAndWatchServicePoolData()
	//if err != nil {
	//	return err
	//}
	return nil
}

func (ss *SystemService) KeyForServicePool() string {
	return fmt.Sprintf("%v.%v.%v", ss.ServiceGeneralConfig.Service.Namespace, KEY_C_COMPONENTS, KEY_C_SERVICE_POOL)
}

func (ss *SystemService) SyncAndWatchServicePoolData() error {
	//rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(context.Background(), "hello", "hello")
	// read service pool data from registry
	getRsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(
		context.Background(), ss.KeyForServicePool())
	if err != nil {
		return err
	}
	// unmarshal json data from response
	if getRsp != nil && getRsp.Count > 0 && len(getRsp.Kvs) > 0 {
		err = json.Unmarshal(getRsp.Kvs[0].Value, ss.ServicePool)
		if err != nil {
			return err
		}
	} else {
		ss.ServicePool = &ServicePool{}
	}
	// fill registry data
	if _, contain := ss.ServicePool.ServerList[ss.ServiceInfo.ServiceTag]; !contain {
		// This service is not currently registered
		ss.ServicePool.ServerList[ss.ServiceInfo.ServiceTag] = &ServiceItem{
			ServiceIntroduce: ss.ServiceInfo.ServiceIntroduce,
		}
	}
	// watch etcd data change
	return nil
}
