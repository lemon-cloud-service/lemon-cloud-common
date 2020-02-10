package lccc_micro_service

type SystemService struct {
}

type ServiceRegisterConfig struct {
	EtcdEndpoints []string
	EtcdUsername  string
	EtcdPassword  string
}

func (ss *SystemService) RegisterNewService(config *ServiceRegisterConfig) error {

}
