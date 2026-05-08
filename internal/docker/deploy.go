package docker

import (
	"fmt"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

func Deploy(dep ctypes.Deployment) error {
	fmt.Print("Deployed!")
	return nil
}
