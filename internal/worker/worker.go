package worker

import (
	"context"
	"fmt"
	"log"

	postgresdb "github.com/scythe504/fluxstream/internal/postgres-db"
	redisdb "github.com/scythe504/fluxstream/internal/redis-db"
	"github.com/scythe504/fluxstream/internal/storage"
	"github.com/scythe504/fluxstream/internal/tor"
)

type TorrentWorker struct {
	rdb        redisdb.Service
	postgresdb postgresdb.Service
	tor        tor.Torrent
	st         storage.Service
	jobsChan   chan redisdb.Job
	numWorker  int
	errChan    chan WorkerError
	ctx        context.Context
}

type WorkerError struct {
	JobId string
	Err   error
	Phase ErrPhase
}

type ErrPhase string

const (
	MAGNET              ErrPhase = "magnet_link_add"
	UPDATE_FAILED       ErrPhase = "update_failed"
	DOWNLOAD_FAILED     ErrPhase = "download_failed"
	BUCKET_WRITE_ERR    ErrPhase = "bucket_write_err"
	TORRENT_CLEANUP_ERR ErrPhase = "torrent_cleanup_err"
	METADATA_FETCH_ERR  ErrPhase = "metadata_fetch_err"
)

func NewTorrentWorker(worker int) *TorrentWorker {
	ctx := context.Background()

	jobsChan := make(chan redisdb.Job, worker)
	errChan := make(chan WorkerError, worker)
	tw := &TorrentWorker{
		rdb:        redisdb.New(ctx),
		postgresdb: postgresdb.New(),
		tor:        tor.New(42070),
		st:         storage.New(),
		jobsChan:   jobsChan,
		ctx:        ctx,
		numWorker:  worker,
		errChan:    errChan,
	}

	return tw
}

func (tw *TorrentWorker) Start(consumerName string) {
	for {
		job, err := tw.rdb.ConsumeJob(tw.ctx, consumerName)

		if err != nil {
			log.Printf("[%s] ConsumeJob error: %v\n", consumerName, err)
			continue
		}

		if job == nil {
			continue
		}

		if err := tw.postgresdb.UpdateStatus(postgresdb.DOWNLOADING, job.Id, nil); err != nil {
			log.Printf("[%s] UpdateStatus DOWNLOADING failed: %v\n", consumerName, err)
			continue
		}

		tw.jobsChan <- *job
	}
}

func (tw *TorrentWorker) DownloadWorker(i int) {
	for job := range tw.jobsChan {
		tw.processJob(job)
	}
}

func (tw *TorrentWorker) processJob(job redisdb.Job) {
	// 1. Add torrent
	if err := tw.tor.AddMagnet(job.Id, job.Link); err != nil {
		tw.errChan <- WorkerError{
			JobId: job.Id,
			Err:   err,
			Phase: MAGNET,
		}
		return
	}

	defer func() {
		// Cleanup torrent connection
		if err := tw.tor.CleanupTorrent(job.Id); err != nil {
			tw.errChan <- WorkerError{
				JobId: job.Id,
				Err:   err,
				Phase: TORRENT_CLEANUP_ERR,
			}
		}
	}()

	// 4. Get file reader
	reader := tw.tor.GetReader(job.Id)
	if reader == nil {
		tw.errChan <- WorkerError{
			JobId: job.Id,
			Err:   fmt.Errorf("torrent reader not found"),
			Phase: DOWNLOAD_FAILED,
		}
		return
	}
	defer (*reader).Close()

	metadata, err := tw.tor.GetMetadata(job.Id)

	if err != nil {
		tw.errChan <- WorkerError{
			JobId: job.Id,
			Err:   err,
			Phase: METADATA_FETCH_ERR,
		}
		return
	}

	// 5. Save video file to storage
	filepath, err := tw.st.SaveForLater(job.Id, *reader, *metadata)
	if err != nil {
		tw.errChan <- WorkerError{
			JobId: job.Id,
			Err:   err,
			Phase: BUCKET_WRITE_ERR,
		}
		return
	}

	// 6. Update DB with file path
	if err := tw.postgresdb.UpdateStatus(postgresdb.DOWNLOADED, job.Id, &filepath); err != nil {
		tw.errChan <- WorkerError{
			JobId: job.Id,
			Err:   err,
			Phase: UPDATE_FAILED,
		}
		return
	}

}

func (tw *TorrentWorker) HandleErrors() {
	for we := range tw.errChan {
		log.Printf("[ERROR] JobId: %s, Phase: %s, Error: %v\n", we.JobId, we.Phase, we.Err)

		// Update DB status to FAILED
		if err := tw.postgresdb.UpdateStatus(postgresdb.FAILED, we.JobId, nil); err != nil {
			log.Printf("[ERROR] Failed to update status to FAILED for jobId %s: %v\n", we.JobId, err)
		}
	}
}
