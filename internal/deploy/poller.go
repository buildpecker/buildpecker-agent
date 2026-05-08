package deploy

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pthsarmah/forge-agent/internal/api"
	ctypes "github.com/pthsarmah/forge-agent/types"
)

func Start(ctx context.Context) error {

	jobs := make(chan ctypes.Job, 100)

	//workers (10 for now)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				Handler(job.Action, job.Data)
			}
		}(i)
	}

	//poller ticker
	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			deps, err := api.GetQueuedDeployments()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in getting queued deployments: %v", err)
				continue
			}
			for _, dep := range deps {

				var job ctypes.Job

				if dep.Project.Framework == "" {
					job = ctypes.Job{
						Action: "start_detect",
						Data:   dep,
					}
				} else {
					job = ctypes.Job{
						Action: "start_deploy",
						Data:   dep,
					}
				}

				select {
				case jobs <- job:
				case <-ctx.Done():
					close(jobs)
					wg.Wait()
					return ctx.Err()
				}
			}
		}
	}
}
