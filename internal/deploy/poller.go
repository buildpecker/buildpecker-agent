package deploy

import (
	"context"
	"sync"
	"time"

	"github.com/pthsarmah/forge-agent/internal/api"
	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func Start(ctx context.Context) error {
	logger, _ := utils.GetLoggerInstance()
	logger.DeployLogger.Println("Deploy poller started")

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
			logger.DeployLogger.Printf("Deploy poller stopping: %v", ctx.Err())
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			deps, err := api.GetQueuedDeployments()
			if err != nil {
				logger.DeployLogger.Printf("Get queued deployments failed: %v", err)
			} else {
				for _, dep := range deps {
					select {
					case jobs <- ctypes.Job{Action: "start_deploy", Data: dep}:
						logger.DeployLogger.Printf("Enqueued deployment %s", dep.Id)
					case <-ctx.Done():
						logger.DeployLogger.Printf("Deploy poller stopping mid-enqueue: %v", ctx.Err())
						close(jobs)
						wg.Wait()
						return ctx.Err()
					}
				}
			}

			deletes, err := api.GetPendingDeletes()
			if err != nil {
				logger.DeployLogger.Printf("Get pending deletes failed: %v", err)
				continue
			}
			for _, dep := range deletes {
				select {
				case jobs <- ctypes.Job{Action: "start_delete", Data: dep}:
					logger.DeployLogger.Printf("Enqueued delete %s", dep.Id)
				case <-ctx.Done():
					logger.DeployLogger.Printf("Deploy poller stopping mid-enqueue: %v", ctx.Err())
					close(jobs)
					wg.Wait()
					return ctx.Err()
				}
			}

			postInstalls, err := api.GetQueuedPostInstalls()
			if err != nil {
				logger.DeployLogger.Printf("Get queued postinstalls failed: %v", err)
				continue
			}
			for _, run := range postInstalls {
				select {
				case jobs <- ctypes.Job{Action: "postinstall", Data: run}:
					logger.DeployLogger.Printf("Enqueued postinstall %s", run.Id)
				case <-ctx.Done():
					logger.DeployLogger.Printf("Deploy poller stopping mid-enqueue: %v", ctx.Err())
					close(jobs)
					wg.Wait()
					return ctx.Err()
				}
			}
		}
	}
}
