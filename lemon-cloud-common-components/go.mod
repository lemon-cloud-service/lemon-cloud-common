module github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-components

go 1.13

require (
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/jinzhu/gorm v1.9.12
	github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils v0.0.0-00010101000000-000000000000
	github.com/micro/go-micro/v2 v2.1.0
	github.com/micro/go-plugins/registry/etcdv3/v2 v2.0.2
	github.com/prometheus/client_golang v1.4.1 // indirect
	go.etcd.io/etcd v3.3.18+incompatible
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace github.com/lemon-cloud-service/lemon-cloud-common/lemon-cloud-common-utils => ../lemon-cloud-common-utils

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
