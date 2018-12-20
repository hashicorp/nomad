package drivers

import "fmt"

var ErrTaskNotFound = fmt.Errorf("task not found for given id")

var DriverRequiresRootMessage = "Driver must run as root"
