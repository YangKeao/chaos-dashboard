module github.com/YangKeao/chaos-dashboard

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-sql-driver/mysql v1.4.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/pingcap/chaos-operator v0.0.0-20191220132106-dcd74a77cdaf
	k8s.io/api v0.0.0-20191121015604-11707872ac1c
	k8s.io/apimachinery v0.0.0-20191121015412-41065c7a8c2a
	k8s.io/client-go v0.0.0-20191121015835-571c0ef67034
	sigs.k8s.io/controller-runtime v0.4.0
)

replace github.com/pingcap/chaos-operator => /home/yangkeao/Project-2019/chaos-operator
