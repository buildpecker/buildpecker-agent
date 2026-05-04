package deploy

import "time"

func Start() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {

	}
}
