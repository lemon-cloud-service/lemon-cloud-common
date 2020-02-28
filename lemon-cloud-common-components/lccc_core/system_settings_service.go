package lccc_core

import (
	"fmt"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_general_manager"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components/lccc_model"
	"github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils/lccu_log"
	"github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/source/etcd"
	"sync"
)

type SystemSettingsServiceStruct struct {
	SystemSettingsContext         config.Config
	SystemSettingsGroupDefinePool map[string]*lccc_model.SystemSettingGroupDefine
	SystemSettingsItemDefinePool  map[string]map[string]*lccc_model.SystemSettingItemDefine
}

var systemSettingsServiceInstance *SystemSettingsServiceStruct
var systemSettingsServiceOnce sync.Once

// 单例函数
func SystemSettingsService() *SystemSettingsServiceStruct {
	systemSettingsServiceOnce.Do(func() {
		systemSettingsServiceInstance = &SystemSettingsServiceStruct{}
		// 将系统设置定义缓存到Map中，方便提高后面的效率
		systemSettingsServiceInstance.SystemSettingsGroupDefinePool = make(map[string]*lccc_model.SystemSettingGroupDefine)
		systemSettingsServiceInstance.SystemSettingsItemDefinePool = make(map[string]map[string]*lccc_model.SystemSettingItemDefine)
		for _, systemSettingsGroupDefine := range CoreService().CoreStartParams.SystemSettingsDefine.SettingGroupList {
			systemSettingsServiceInstance.SystemSettingsGroupDefinePool[systemSettingsGroupDefine.Key] = systemSettingsGroupDefine
			systemSettingsServiceInstance.SystemSettingsItemDefinePool[systemSettingsGroupDefine.Key] = make(map[string]*lccc_model.SystemSettingItemDefine)
			for _, systemSettingItemDefine := range systemSettingsGroupDefine.SettingList {
				systemSettingsServiceInstance.SystemSettingsItemDefinePool[systemSettingsGroupDefine.Key][systemSettingItemDefine.Key] = systemSettingItemDefine
			}
		}
	})
	return systemSettingsServiceInstance
}

func (sss *SystemSettingsServiceStruct) Init() error {
	var err error
	if err = sss.AutoFixSystemSettingsValue(); err != nil {
		return err
	}
	if sss.SystemSettingsContext, err = config.NewConfig(); err != nil {
		return err
	}
	etcdSource := etcd.NewSource(
		etcd.WithAddress(CoreService().CoreStartParams.ServiceGeneralConfig.GetRegistryUrl()),
		etcd.WithPrefix(fmt.Sprintf("/%v/%v/%v", KEY_C_SYSTEM_SETTINGS_VALUE, CoreService().CoreStartParams.ServiceGeneralConfig.Service.Namespace, CoreService().CoreStartParams.ServiceBaseInfo.ServiceKey)),
		etcd.StripPrefix(true))
	if err = sss.SystemSettingsContext.Load(etcdSource); err != nil {
		return err
	}
	return nil
}

func (sss *SystemSettingsServiceStruct) AutoFixSystemSettingsValue() error {
	for _, systemSettingsGroupDefine := range CoreService().CoreStartParams.SystemSettingsDefine.SettingGroupList {
		for _, systemSettingItemDefine := range systemSettingsGroupDefine.SettingList {
			key := fmt.Sprintf("/%v/%v/%v/%v/%v", KEY_C_SYSTEM_SETTINGS_VALUE, CoreService().CoreStartParams.ServiceGeneralConfig.Service.Namespace, CoreService().CoreStartParams.ServiceBaseInfo.ServiceKey, systemSettingsGroupDefine.Key, systemSettingItemDefine.Key)
			keyName := fmt.Sprintf("/%v/%v", systemSettingsGroupDefine.Key, systemSettingItemDefine.Key)
			getRsp, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Get(GetDefaultRegistryContext(), key)
			if err != nil {
				lccu_log.Errorf("[GET] An error occurred while fixing the exact system settings item %v %v.", keyName, err)
			}
			if getRsp.Kvs == nil {
				lccu_log.Infof("The missing of the specified system setting item %v is detected, and the default value is added: %v", keyName, systemSettingItemDefine.DefaultValue)
				if _, err := lccc_general_manager.EtcdManagerInstance().ClientInstance().Put(GetDefaultRegistryContext(), key, systemSettingItemDefine.DefaultValue); err != nil {
					lccu_log.Errorf("[PUT] An error occurred while fixing the exact system settings item %v %v.", keyName, err)
				}
			}
		}
	}
	return nil
}

func (sss *SystemSettingsServiceStruct) KGet(key lccc_model.SystemSettingItemKeyDefine) string {
	return sss.Get(key.GroupKey, key.GroupKey)
}

func (sss *SystemSettingsServiceStruct) Get(settingsGroupKey, settingItemKey string) string {
	return sss.SystemSettingsContext.Get(settingsGroupKey, settingItemKey).String(sss.GetSystemSettingItemDefaultValue(settingsGroupKey, settingItemKey))
}

func (sss *SystemSettingsServiceStruct) KSet(key lccc_model.SystemSettingItemKeyDefine, newValue string) {
	sss.Set(key.GroupKey, key.ItemKey, newValue)
}

func (sss *SystemSettingsServiceStruct) Set(settingsGroupKey, settingItemKey, newValue string) {
	sss.SystemSettingsContext.Set(newValue, settingsGroupKey, settingItemKey)
}

func (sss *SystemSettingsServiceStruct) KWatch(key lccc_model.SystemSettingItemKeyDefine, callback func(string)) {
	sss.Watch(key.GroupKey, key.ItemKey, callback)
}

func (sss *SystemSettingsServiceStruct) Watch(settingsGroupKey, settingItemKey string, callback func(string)) {
	watch, err := sss.SystemSettingsContext.Watch(settingsGroupKey, settingItemKey)
	lccu_log.Error("An error occurred while observing the system settings：", err)
	go func() {
		if val, err := watch.Next(); err == nil {
			callback(val.String(sss.GetSystemSettingItemDefaultValue(settingsGroupKey, settingItemKey)))
		} else {
			lccu_log.Error("An error occurred while observing the change of system settings：", err)
		}
		sss.Watch(settingsGroupKey, settingItemKey, callback)
	}()
}

// 获取系统设置定义实例，如果事先没定义过这个分组，那么将返回nil
func (sss *SystemSettingsServiceStruct) GetSystemSettingsGroupDefine(settingsGroupKey string) *lccc_model.SystemSettingGroupDefine {
	if systemSettingsGroup, ok := sss.SystemSettingsGroupDefinePool[settingsGroupKey]; ok {
		return systemSettingsGroup
	}
	return nil
}

// 获取系统设置子项定义实例，如果事先没定义过这个系统设置项，那么将返回nil
func (sss *SystemSettingsServiceStruct) GetSystemSettingItemDefine(settingsGroupKey, settingItemKey string) *lccc_model.SystemSettingItemDefine {
	if settingsItemMap, ok := sss.SystemSettingsItemDefinePool[settingsGroupKey]; ok {
		if settingItemDefine, ok := settingsItemMap[settingItemKey]; ok {
			return settingItemDefine
		}
	}
	return nil
}

// 获取系统设置子项定义的默认值，如果事先没定义过这个系统设置项，那么将返回空字符串
func (sss *SystemSettingsServiceStruct) GetSystemSettingItemDefaultValue(settingsGroupKey, settingItemKey string) string {
	if settingItemDefine := sss.GetSystemSettingItemDefine(settingsGroupKey, settingItemKey); settingItemDefine != nil {
		return settingItemDefine.DefaultValue
	}
	return ""
}
