package usecases

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
	"github.com/Mersad-Moghaddam/shared/messagequeue"
)

// WorkerManager manages a pool of workers for processing messages
type WorkerManager struct {
	fanoutUseCase   ports.FanoutUsecase
	rabbitMQ        *messagequeue.RabbitMQ
	concurrency     int
	batchSize       int
	workers         []*Worker
	stopChan        chan struct{}
	workerWaitGroup sync.WaitGroup
}

// Worker represents a single worker process
type Worker struct {
	id            int
	fanoutUseCase ports.FanoutUsecase
	rabbitMQ      *messagequeue.RabbitMQ
	batchSize     int
	stopChan      chan struct{}
	metrics       *WorkerMetrics
}

// WorkerMetrics tracks metrics for a worker
type WorkerMetrics struct {
	messagesProcessed int
	batchesProcessed  int
	errors            int
	lastProcessedAt   time.Time
	processingTimes   []time.Duration
	mutex             sync.Mutex
}

// IncrementProcessed increments the processed messages counter
func (wm *WorkerMetrics) IncrementProcessed() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.messagesProcessed++
}

// IncrementErrors increments the error counter
func (wm *WorkerMetrics) IncrementErrors() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.errors++
}

// RecordProcessingTime records the processing time for a message
func (wm *WorkerMetrics) RecordProcessingTime(duration time.Duration) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.processingTimes = append(wm.processingTimes, duration)
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(fanoutUseCase ports.FanoutUsecase, rabbitMQ *messagequeue.RabbitMQ, concurrency, batchSize int) *WorkerManager {
	return &WorkerManager{
		fanoutUseCase:   fanoutUseCase,
		rabbitMQ:        rabbitMQ,
		concurrency:     concurrency,
		batchSize:       batchSize,
		stopChan:        make(chan struct{}),
		workers:         make([]*Worker, 0, concurrency),
		workerWaitGroup: sync.WaitGroup{},
	}
}

// Start initializes and starts all workers
func (wm *WorkerManager) Start() {
	wm.workers = make([]*Worker, wm.concurrency)

	for i := 0; i < wm.concurrency; i++ {
		worker := &Worker{
			id:            i,
			fanoutUseCase: wm.fanoutUseCase,
			rabbitMQ:      wm.rabbitMQ,
			batchSize:     wm.batchSize,
			stopChan:      wm.stopChan,
			metrics: &WorkerMetrics{
				lastProcessedAt: time.Now(),
			},
		}

		wm.workers[i] = worker
		wm.workerWaitGroup.Add(1)

		go func(w *Worker) {
			defer wm.workerWaitGroup.Done()
			w.start()
		}(worker)
	}

	// Start metrics reporter
	go wm.ReportMetrics()
}

// Stop signals all workers to stop and waits for them to finish
func (wm *WorkerManager) Stop() {
	close(wm.stopChan)
	wm.workerWaitGroup.Wait()
}

// ReportMetrics reports the current metrics for all workers
func (wm *WorkerManager) ReportMetrics() {
	var totalMessages int
	var totalBatches int
	var totalErrors int

	for _, worker := range wm.workers {
		worker.metrics.mutex.Lock()
		totalMessages += worker.metrics.messagesProcessed
		totalBatches += worker.metrics.batchesProcessed
		totalErrors += worker.metrics.errors
		worker.metrics.mutex.Unlock()
	}

	log.Printf("Worker Metrics - Messages Processed: %d, Batches Processed: %d, Errors: %d",
		totalMessages, totalBatches, totalErrors)
}

// start begins the worker's processing loop
func (w *Worker) start() {
	log.Printf("Worker %d started", w.id)

	// Handle post events for fanout
	err := w.rabbitMQ.ConsumeMessages("post_events", w.handleMessage)
	if err != nil {
		log.Printf("Worker %d failed to consume messages: %v", w.id, err)
		return
	}

	<-w.stopChan
	log.Printf("Worker %d stopped", w.id)
}

// ProcessMessages processes messages from the queue
func (w *Worker) ProcessMessages() {
	log.Printf("Worker %d started", w.id)

	// Set up a handler for processing messages
	err := w.rabbitMQ.ConsumeMessages("post_events", func(message messagequeue.Message) error {
		startTime := time.Now()

		// Check if this is a PostCreated event
		if message.Type == messagequeue.PostCreated {
			// Extract post data
			var postData struct {
				ID       uint `json:"id"`
				AuthorID uint `json:"author_id"`
			}

			if err := json.Unmarshal(message.Data, &postData); err != nil {
				log.Printf("Worker %d error unmarshaling post data: %v", w.id, err)
				w.metrics.IncrementErrors()
				return err
			}

			// Call fanout use case with enhanced processing
			if err := w.fanoutUseCase.ProcessPostCreated(postData.ID, postData.AuthorID); err != nil {
				log.Printf("Worker %d error processing fanout: %v", w.id, err)
				w.metrics.IncrementErrors()
				return err
			}

			processingTime := time.Since(startTime)
			w.metrics.RecordProcessingTime(processingTime)
			w.metrics.IncrementProcessed()
			log.Printf("Worker %d processed post %d in %v", w.id, postData.ID, processingTime)
		}

		return nil
	})

	if err != nil {
		log.Printf("Worker %d error setting up consumer: %v", w.id, err)
		return
	}

	// Wait for stop signal
	<-w.stopChan
	log.Printf("Worker %d stopping", w.id)
}

// PostEvent represents a post creation event
type PostEvent struct {
	PostID   uint `json:"post_id"`
	UserID   uint `json:"user_id"`
	AuthorID uint `json:"author_id"`
}

// handleMessage processes a message from the queue
func (w *Worker) handleMessage(message messagequeue.Message) error {
	w.metrics.mutex.Lock()
	w.metrics.messagesProcessed++
	w.metrics.lastProcessedAt = time.Now()
	w.metrics.mutex.Unlock()

	if message.Type != messagequeue.PostCreated {
		return nil // Only process post created events
	}

	var postEvent PostEvent
	if err := json.Unmarshal(message.Data, &postEvent); err != nil {
		w.metrics.mutex.Lock()
		w.metrics.errors++
		w.metrics.mutex.Unlock()
		return err
	}

	// Process the post event with fanout logic
	err := w.fanoutUseCase.ProcessPostCreated(postEvent.PostID, postEvent.AuthorID)
	if err != nil {
		w.metrics.mutex.Lock()
		w.metrics.errors++
		w.metrics.mutex.Unlock()
		return err
	}

	w.metrics.mutex.Lock()
	w.metrics.batchesProcessed++
	w.metrics.mutex.Unlock()

	return nil
}
