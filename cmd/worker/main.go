package main

import (
	"fmt"

	"github.com/scythe504/fluxstream/internal/worker"
)

const (
	workers int = 1
)

func main() {

	tw := worker.NewTorrentWorker(workers)

	for i := range workers {
		workerName := fmt.Sprintf("worker-%d", i)
		go tw.Start(workerName)
	}

	for i := range workers {
		go tw.DownloadWorker(i)
	}

	go tw.HandleErrors()
	select {}
}
