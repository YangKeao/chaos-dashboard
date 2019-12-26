package util

import (
	"fmt"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	InitLog            = ctrl.Log.WithName("setup")
	DashboardNamespace string
	DataSource         string
)

func init() {
	var ok bool

	DashboardNamespace, ok = os.LookupEnv("NAMESPACE")
	if !ok {
		InitLog.Error(nil, "cannot find NAMESPACE")
		DashboardNamespace = "chaos"
	}

	DataSource = fmt.Sprintf("root:@tcp(chaos-collector-database.%s:3306)/chaos_operator", DashboardNamespace)
}
