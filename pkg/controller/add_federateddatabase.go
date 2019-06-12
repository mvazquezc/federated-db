package controller

import (
	"github.com/mvazquezc/federated-db/pkg/controller/federateddatabase"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, federateddatabase.Add)
}
