package scheduler

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/database" // Module path
)

// ScheduledTask represents a task in the priority queue.
type ScheduledTask struct {
	Feed      *database.Feed
	NextRun   time.Time
	index     int // Index in the heap.
	taskFunc  func(f *database.Feed)
}

// PriorityQueue implements heap.Interface and holds ScheduledTasks.
type PriorityQueue []*ScheduledTask

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the task with the earliest NextRun time.
	return pq[i].NextRun.Before(pq[j].NextRun)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds an item to the priority queue.
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*ScheduledTask)
	item.index = n
	*pq = append(*pq, item)
}

// Pop removes and returns the item with the earliest NextRun time.
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// FeedScheduler manages feed fetching schedules.
type FeedScheduler struct {
	pq      PriorityQueue
	mu      sync.Mutex
	timer   *time.Timer
	stopCh  chan struct{}
	running bool
}

// NewFeedScheduler creates a new scheduler.
func NewFeedScheduler() *FeedScheduler {
	return &FeedScheduler{
		pq:     make(PriorityQueue, 0),
		stopCh: make(chan struct{}),
	}
}

// Add schedules a feed for periodic fetching.
func (s *FeedScheduler) Add(feed *database.Feed, taskFunc func(f *database.Feed)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if feed.FrequencySeconds <= 0 {
		feed.FrequencySeconds = 300 // Default to 5 minutes if invalid
		log.Warn().Int64("feed_id", feed.ID).Str("url", feed.URL).Msg("Feed frequency is zero or negative, defaulting to 5 minutes.")
	}

	// Initial run slightly delayed to distribute load, or immediately if desired.
	// Or, if LastFetchedAt is available, schedule relative to that.
	nextRun := time.Now().Add(5 * time.Second) // Small initial delay
	if feed.LastFetchedAt != nil {
		// Schedule based on last fetch + frequency, but not in the past
		potentialNextRun := feed.LastFetchedAt.Add(time.Duration(feed.FrequencySeconds) * time.Second)
		if potentialNextRun.After(time.Now()){
			nextRun = potentialNextRun
		} else {
			// If it's already due, run soon
			nextRun = time.Now().Add(1 * time.Second) 
		}
	}


	task := &ScheduledTask{
		Feed:     feed,
		NextRun:  nextRun,
		taskFunc: taskFunc,
	}
	heap.Push(&s.pq, task)
	log.Info().Int64("feed_id", feed.ID).Str("url", feed.URL).Time("initial_run_at", nextRun).Msg("Feed added to scheduler")

	if s.running && (s.timer == nil || nextRun.Before(s.pq[0].NextRun)) {
		s.resetTimer()
	}
	return nil
}

// Start begins the scheduler loop.
func (s *FeedScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Info().Msg("Scheduler started")
	s.resetTimer() // Set initial timer

	go func() {
		for {
			select {
			case <-s.stopCh:
				log.Info().Msg("Scheduler stopping...")
				if s.timer != nil {
					s.timer.Stop()
				}
				s.mu.Lock()
				s.running = false
				s.mu.Unlock()
				log.Info().Msg("Scheduler stopped")
				return
			case <-s.timer.C: // timer will be nil if pq is empty
				s.runPendingTasks()
				s.resetTimer()
			}
		}
	}()
}

func (s *FeedScheduler) runPendingTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for s.pq.Len() > 0 {
		task := s.pq[0] // Peek
		if task.NextRun.After(now) {
			break // Not yet time for this task
		}

		heap.Pop(&s.pq) // Remove it

		log.Debug().Int64("feed_id", task.Feed.ID).Str("url", task.Feed.URL).Msg("Executing scheduled task")
		go task.taskFunc(task.Feed) // Run task in a new goroutine

		// Reschedule for next run
		task.NextRun = now.Add(time.Duration(task.Feed.FrequencySeconds) * time.Second)
		heap.Push(&s.pq, task)
		log.Debug().Int64("feed_id", task.Feed.ID).Time("next_run_at", task.NextRun).Msg("Feed rescheduled")
	}
}

func (s *FeedScheduler) resetTimer() {
	// This function MUST be called with s.mu locked if s.running is true,
	// or before s.running is set to true during Start.
	if !s.running { // If called from Add before Start
		return
	}

	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil // Ensure old timer channel is not read
	}

	if s.pq.Len() == 0 {
		log.Debug().Msg("Scheduler queue is empty, timer not set.")
		// Create a dummy timer that will never fire, or handle this case specially
		// For simplicity, we can just let it be nil. The select will block.
		// Or set a very long timer to prevent busy loop on stopCh
		s.timer = time.NewTimer(24 * time.Hour) // Effectively idle
		return
	}

	nextRunDelay := s.pq[0].NextRun.Sub(time.Now())
	if nextRunDelay < 0 {
		nextRunDelay = 0 // Run immediately if overdue
	}
	
	s.timer = time.NewTimer(nextRunDelay)
	log.Debug().Dur("next_timer_fire_in", nextRunDelay).Msg("Scheduler timer reset")
}


// Stop signals the scheduler to halt.
func (s *FeedScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	// Wait for running to be false? Or just signal?
	// For graceful shutdown, might need a WaitGroup or similar.
	s.mu.Unlock()
	log.Info().Msg("Scheduler stop signal sent")
}