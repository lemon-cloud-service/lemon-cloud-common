package lccc_micro_service

import (
	"context"
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_log"
	"go.etcd.io/etcd/clientv3"
	"strings"
	"sync"
)

type SystemSettingsService struct {
	SystemSettingsValueChangeListenerPool map[string]map[string]func(string) // 系统设置值变动监听器存储池
	// 缓存
	AllSettingsValueCache map[string]map[string]string `json:"all_settings_value_cache"`
}

var systemSettingsServiceInstance *SystemSettingsService
var systemSettingsServiceOnce sync.Once

// 单例函数
func SystemSettingsSviceSingletonInstance() *SystemSettingsService {
	systemSettingsServiceOnce.Do(func() {
		systemSettingsServiceInstance = &SystemSettingsService{}
		_, err := systemSettingsServiceInstance.GetAllSystemSettingsValue()
		if err != nil {
			lccu_log.Error("Initially read system settings value failed, reason: ", err.Error())
		}
		systemSettingsServiceInstance.StartWatchCurrentServiceSettingsValue()
	})
	return systemSettingsServiceInstance
}

// 开始监听当前服务的系统设置值的变动，如有变动更新缓存
func (sss *SystemSettingsService) StartWatchCurrentServiceSettingsValue() {
	watchKey := fmt.Sprintf("%v.%v.%v",
		CoreServiceSingletonInstance().ServiceGeneralConfig.Service.Namespace,
		KEY_C_SYSTEM_SETTINGS_VALUE,
		CoreServiceSingletonInstance().ServiceBaseInfo.ServiceKey)
	watchChain := lccc_general_manager.EtcdManagerInstance().ClientInstance().Watch(context.Background(), watchKey, clientv3.WithPrefix())
	go func() {
		for range watchChain {
			lccu_log.Info("Observe that the system settings have changed and start refreshing the data...")
			var err error = nil
			_, err = sss.GetAllSystemSettingsValue()
			if err != nil {
				lccu_log.Error("Observed a change in the system settings value of the registry, but failed to refresh the system settings value data, reason: ", err.Error())
			}
		}
	}()
}

// 快速从缓存中获取所有系统设置项值
func (sss *SystemSettingsService) FastGetAllSystemSettingsValue() map[string]map[string]string {
	return sss.AllSettingsValueCache
}

// 从注册中心获取所有系统设置项值，忽略缓存，直接从注册中心重新获取数据
func (sss *SystemSettingsService) GetAllSystemSettingsValue() (map[string]map[string]string, error) {
	lccu_log.Info("Start refresh all system settings value data directly from the registry...")
	prefixKey := fmt.Sprintf("%v.%v.%v",
		CoreServiceSingletonInstance().ServiceGeneralConfig.Service.Namespace,
		KEY_C_SYSTEM_SETTINGS_VALUE,
		CoreServiceSingletonInstance().ServiceBaseInfo.ServiceKey)
	rsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(context.Background(), prefixKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	valuePool := make(map[string]map[string]string)
	for _, kvs := range rsp.Kvs {
		fullKey := fmt.Sprintf("%s", kvs.Key)
		fullKeyComponents := strings.Split(fullKey, ".")
		if len(fullKeyComponents) < 5 {
			lccu_log.Error("The configuration key is invalid, skip and delete it! The format should be: $namespace.system_settings_value.$service_key.$settings_group_key.$settings_item_key, but current key is: %s", fullKey)
			if _, err = lccc_general_manager.EtcdManagerInstance().ClientInstance().Delete(context.Background(), fullKey); err != nil {
				lccu_log.Error("Invalid system settings key deletion failed, reason: %v", err.Error())
			}
			continue
		}
		groupKey := fullKeyComponents[len(fullKeyComponents)-2]
		if _, ok := valuePool[groupKey]; !ok {
			// groupKey 不存在，创建一个groupKey对应的map
			valuePool[groupKey] = make(map[string]string)
		}
		itemKey := fullKeyComponents[len(fullKeyComponents)-1]
		itemValue := fmt.Sprintf("%s", kvs.Value)
		valuePool[groupKey][itemKey] = itemValue
		if group, ok := sss.SystemSettingsValueChangeListenerPool[groupKey]; ok {
			if callback, ok := group[itemKey]; ok {
				// 注册了监听函数
				callback(itemValue)
			}
		}
	}
	sss.FixSettingValuePool(valuePool, CoreServiceSingletonInstance().SystemSettingsDefine)
	// save to cache
	sss.AllSettingsValueCache = valuePool
	lccu_log.Info("Refreshing all system settings value data succeeded")
	return valuePool, nil
}

// 修复从Registry中获取到的系统设置项值的map，对缺失的配置项，如果没有的话那么填充进去，如果未曾设置过，那么赋予default值
func (sss *SystemSettingsService) FixSettingValuePool(valuePool map[string]map[string]string, settingsDefine *lccc_model.SystemSettingsDefine) {
	for _, group := range settingsDefine.SettingGroupList {
		if _, ok := valuePool[group.Key]; !ok {
			valuePool[group.Key] = make(map[string]string)
		}
		for _, item := range group.SettingList {
			if _, ok := valuePool[group.Key][item.Key]; !ok {
				valuePool[group.Key][item.Key] = item.DefaultValue
			}
		}
	}
}

// 设置系统设置值改变监听器
func (sss *SystemSettingsService) SetSystemSettingsValueChangeListener(settingGroupKey, settingItemKey string, callback func(string)) {
	if _, ok := sss.SystemSettingsValueChangeListenerPool[settingGroupKey]; !ok {
		sss.SystemSettingsValueChangeListenerPool[settingGroupKey] = make(map[string]func(string))
	}
	sss.SystemSettingsValueChangeListenerPool[settingGroupKey][settingItemKey] = callback
}
