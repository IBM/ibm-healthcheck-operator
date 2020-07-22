package controller

import (
	"github.com/IBM/ibm-healthcheck-operator/pkg/controller/mustgatherjob"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, mustgatherjob.Add)
}
