package controller

import (
	"github.ibm.com/IBMPrivateCloud/health-service-operator/pkg/controller/healthservice"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, healthservice.Add)
}
