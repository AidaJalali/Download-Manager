package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mahdiXak47/Download-Manager/internal/logger"
)

// Download represents a download task with its state and control channels
type Download struct {
	URL                string    `json:"url"`
	TargetPath         string    `json:"target_path"`
	Filename           string    `json:"filename"`
	Queue              string    `json:"queue"`
	Status             string    `json:"status"` // pending, downloading, paused, completed, error, cancelled
	Progress           float64   `json:"progress"`
	Speed              int64     `json:"speed"` // bytes per second
	TotalSize          int64     `json:"total_size"`
	Downloaded         int64     `json:"downloaded"`
	Error              string    `json:"error,omitempty"`
	MaxBandwidth       int64     `json:"max_bandwidth"` // in KB/s, 0 means unlimited
	StartTime          time.Time `json:"start_time,omitempty"`
	CompletionTime     time.Time `json:"completion_time,omitempty"`
	ScheduledStartTime time.Time `json:"scheduled_start_time,omitempty"`

	// Control fields (not persisted to JSON)
	pauseChan      chan struct{} `json:"-"`
	resumeChan     chan struct{} `json:"-"`
	cancelChan     chan struct{} `json:"-"`
	isPaused       bool          `json:"-"`
	isCancelled    bool          `json:"-"`
	mutex          sync.Mutex    `json:"-"`
	RetryCount     int           `json:"retry_count"`
	MaxRetries     int           `json:"max_retries"`
	RetryDelay     time.Duration `json:"-"`
	client         *http.Client  `json:"-"`
	supportsRanges bool          `json:"-"`
}

// DownloadResult represents the outcome of a download attempt
type DownloadResult struct {
	Completed   bool
	Downloaded  int64
	TotalSize   int64
	Error       error
	ShouldRetry bool
}

// Initialize sets up control channels for a download
func (d *Download) Initialize() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.pauseChan == nil {
		d.pauseChan = make(chan struct{}, 1)
	}
	if d.resumeChan == nil {
		d.resumeChan = make(chan struct{}, 1)
	}
	if d.cancelChan == nil {
		d.cancelChan = make(chan struct{}, 1)
	}
	if d.Status == "" {
		d.Status = "pending"
		logger.LogDownloadPending(d.URL, d.Queue, "Initialized download")
	}
	if d.MaxRetries == 0 {
		d.MaxRetries = 3
	}
	if d.RetryDelay == 0 {
		d.RetryDelay = 5 * time.Second
	}
	if d.client == nil {
		// Configure HTTP client with more lenient timeouts
		d.client = &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   30 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				ExpectContinueTimeout: 5 * time.Second,
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
				DisableCompression:    false,
			},
		}
	}
	if d.Filename == "" && d.URL != "" {
		d.Filename = filepath.Base(d.URL)
	}
	if d.TargetPath == "" && d.Filename != "" {
		d.TargetPath = d.Filename
	}
}

// Pause signals the download to pause
func (d *Download) Pause() {
	d.mutex.Lock()
	oldStatus := d.Status
	defer d.mutex.Unlock()

	if d.Status == "downloading" && !d.isPaused && !d.isCancelled {
		d.Status = "paused"
		d.isPaused = true
		// Log status change to paused
		logger.LogDownloadStatus(d.URL, oldStatus, d.Status, d.Downloaded, d.TotalSize)
		select {
		case d.pauseChan <- struct{}{}:
		default:
		}
	}
}

// Resume signals the download to resume
func (d *Download) Resume() {
	d.mutex.Lock()
	oldStatus := d.Status
	defer d.mutex.Unlock()

	if d.Status == "paused" && d.isPaused && !d.isCancelled {
		d.Status = "downloading"
		d.isPaused = false
		// Log status change to downloading (resumed)
		logger.LogDownloadStatus(d.URL, oldStatus, d.Status, d.Downloaded, d.TotalSize)
		select {
		case d.resumeChan <- struct{}{}:
		default:
		}
	}
}

// Cancel stops the download and removes temporary files
func (d *Download) Cancel() error {
	d.mutex.Lock()
	oldStatus := d.Status
	defer d.mutex.Unlock()

	if d.Status != "completed" && d.Status != "cancelled" && !d.isCancelled {
		d.Status = "cancelled"
		d.isCancelled = true
		// Log status change to cancelled
		logger.LogDownloadStatus(d.URL, oldStatus, d.Status, d.Downloaded, d.TotalSize)
		select {
		case d.cancelChan <- struct{}{}:
		default:
		}

		// Only attempt to remove the file if it was created
		if d.TargetPath != "" && d.Progress > 0 {
			if err := os.Remove(d.TargetPath); err != nil && !os.IsNotExist(err) {
				errorMsg := fmt.Sprintf("failed to remove file: %v", err)
				logger.LogDownloadError(d.URL, d.Queue, errorMsg)
				return fmt.Errorf("failed to remove file: %v", err)
			}
		}
	}
	return nil
}

// Retry attempts to restart a failed download
func (d *Download) Retry() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.Status == "error" {
		oldStatus := d.Status // Save the old status for logging
		d.Status = "pending"
		d.Error = ""
		d.Progress = 0
		d.Speed = 0
		d.Downloaded = 0
		d.RetryCount++
		// Log status change to pending (retry)
		logger.LogDownloadPending(d.URL, d.Queue, fmt.Sprintf("Retry attempt %d of %d", d.RetryCount, d.MaxRetries))
		// Log the status change
		logger.LogDownloadStatus(d.URL, oldStatus, d.Status, 0, d.TotalSize)
		return nil
	}
	return fmt.Errorf("download is not in error state")
}

// GetStatus returns the current status of the download
func (d *Download) GetStatus() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.Status
}

// GetProgress returns the current progress percentage of the download
func (d *Download) GetProgress() float64 {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.Progress
}

// GetSpeed returns the current download speed in bytes per second
func (d *Download) GetSpeed() int64 {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.Speed
}

// GetRetryCount returns the current retry count for the download
func (d *Download) GetRetryCount() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.RetryCount
}

// Start begins downloading a file with optional progress callback
func (d *Download) Start() error {
	// Initialize control channels and fields
	d.Initialize()
	d.mutex.Lock()
	oldStatus := d.Status
	d.Status = "downloading"
	d.StartTime = time.Now()
	d.mutex.Unlock()

	// Log download start
	logger.LogDownloadStart(d.URL, d.Queue, d.MaxBandwidth)
	// Log status change
	logger.LogDownloadStatus(d.URL, oldStatus, "downloading", 0, d.TotalSize)

	if !d.ScheduledStartTime.IsZero() && time.Now().Before(d.ScheduledStartTime) {
		waitDuration := d.ScheduledStartTime.Sub(time.Now())
		logger.LogDownloadPending(d.URL, d.Queue, fmt.Sprintf("Waiting for scheduled start time: %v", d.ScheduledStartTime))
		time.Sleep(waitDuration)
	}

	// Main download loop with retry logic
	for d.RetryCount <= d.MaxRetries {
		err := d.performDownload()
		if err == nil {
			// Download completed successfully
			d.mutex.Lock()
			oldStatus := d.Status
			d.Status = "completed"
			d.Progress = 100.0
			d.CompletionTime = time.Now()
			d.mutex.Unlock()

			// Calculate download duration
			duration := time.Since(d.StartTime)
			// Log download completion
			logger.LogDownloadComplete(d.URL, d.TargetPath, duration, d.TotalSize)
			// Log status change
			logger.LogDownloadStatus(d.URL, oldStatus, "completed", d.TotalSize, d.TotalSize)
			return nil
		}

		// Check if download was cancelled
		d.mutex.Lock()
		if d.isCancelled {
			d.mutex.Unlock()
			logger.LogDownloadStatus(d.URL, "downloading", "cancelled", d.Downloaded, d.TotalSize)
			return fmt.Errorf("download cancelled")
		}

		// Handle error and retry if possible
		oldStatus := d.Status
		d.Status = "error"
		d.Error = err.Error()

		// Log error status
		logger.LogDownloadError(d.URL, d.Queue, err.Error())
		logger.LogDownloadStatus(d.URL, oldStatus, "error", d.Downloaded, d.TotalSize)

		// Check if we should retry
		if d.RetryCount < d.MaxRetries {
			d.RetryCount++
			d.Status = "pending"
			retryMsg := fmt.Sprintf("Retry attempt %d of %d after error: %s",
				d.RetryCount, d.MaxRetries, err.Error())
			logger.LogDownloadPending(d.URL, d.Queue, retryMsg)
			logger.LogDownloadStatus(d.URL, "error", "pending", d.Downloaded, d.TotalSize)
			d.mutex.Unlock()
			time.Sleep(d.RetryDelay)
			continue
		}

		d.mutex.Unlock()
		finalError := fmt.Errorf("download failed after %d retries: %v", d.MaxRetries, err)
		logger.LogDownloadError(d.URL, d.Queue, finalError.Error())
		return finalError
	}

	finalError := fmt.Errorf("download failed after %d retries", d.MaxRetries)
	logger.LogDownloadError(d.URL, d.Queue, finalError.Error())
	return finalError
}

// performDownload handles the actual file download process
func (d *Download) performDownload() error {
	// Ensure queue name is valid
	if d.Queue == "" {
		d.Queue = "default"
	} else if len(d.Queue) > 50 || d.Queue != filepath.Clean(d.Queue) {
		logger.LogDownloadError(d.URL, d.Queue, "Invalid queue name, using default")
		d.Queue = "default"
	}

	// Try HEAD request first, but don't fail if it doesn't work
	var totalSize int64
	var supportsRanges bool
	var headResp *http.Response

	headResp, err := d.client.Head(d.URL)
	if err != nil {
		// Log the HEAD request failure but don't return error yet
		logger.LogDownloadError(d.URL, d.Queue, fmt.Sprintf("HEAD request failed: %v, proceeding with GET request", err))
	} else {
		defer headResp.Body.Close()
		totalSize, _ = strconv.ParseInt(headResp.Header.Get("Content-Length"), 10, 64)
		supportsRanges = headResp.Header.Get("Accept-Ranges") == "bytes"
	}

	// Create the GET request
	req, err := http.NewRequest("GET", d.URL, nil)
	if err != nil {
		errorMsg := fmt.Sprintf("failed to create request: %v", err)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add some common headers to help with compatibility
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// If we're resuming and we know the server supports ranges, set the range header
	d.mutex.Lock()
	startByte := d.Downloaded
	d.supportsRanges = supportsRanges
	d.mutex.Unlock()

	if startByte > 0 && supportsRanges {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	}

	// Send the request with retry logic
	var getResp *http.Response
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		getResp, err = d.client.Do(req)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			logger.LogDownloadError(d.URL, d.Queue, fmt.Sprintf("GET request attempt %d failed: %v, retrying...", i+1, err))
			time.Sleep(time.Second * time.Duration(i+1)) // Exponential backoff
		}
	}

	if err != nil {
		// Check for network-related errors
		if os.IsTimeout(err) || err == io.ErrUnexpectedEOF || err == io.EOF {
			d.mutex.Lock()
			d.Status = "paused"
			d.isPaused = true
			d.mutex.Unlock()
			logger.LogDownloadStatus(d.URL, "downloading", "paused", d.Downloaded, d.TotalSize)
			return fmt.Errorf("download paused due to network error: %w", err)
		}

		errorMsg := fmt.Sprintf("failed to send GET request after %d attempts: %v", maxRetries, err)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("failed to send GET request: %w", err)
	}
	defer getResp.Body.Close()

	// Check if the request was successful
	if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
		errorMsg := fmt.Sprintf("server responded with status: %s", getResp.Status)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("server responded with status: %s", getResp.Status)
	}

	// Update total size from GET response if we didn't get it from HEAD
	if totalSize == 0 {
		totalSize, _ = strconv.ParseInt(getResp.Header.Get("Content-Length"), 10, 64)
		d.mutex.Lock()
		d.TotalSize = totalSize
		d.supportsRanges = getResp.Header.Get("Accept-Ranges") == "bytes"
		d.mutex.Unlock()
	}

	// If we got a 206 response, the server supports ranges
	if getResp.StatusCode == 206 {
		supportsRanges = true
	}

	// Prepare file for writing
	var file *os.File
	var openMode int

	if startByte > 0 && supportsRanges {
		openMode = os.O_WRONLY | os.O_APPEND
	} else {
		openMode = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		startByte = 0
	}

	// Verify target directory exists and is writable
	dir := filepath.Dir(d.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		errorMsg := fmt.Sprintf("failed to create directory: %v", err)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if we can write to the target directory
	if err := os.Chmod(dir, 0755); err != nil {
		errorMsg := fmt.Sprintf("target directory is not writable: %v", err)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("target directory is not writable: %w", err)
	}

	file, err = os.OpenFile(d.TargetPath, openMode, 0644)
	if err != nil {
		errorMsg := fmt.Sprintf("failed to open file: %v", err)
		logger.LogDownloadError(d.URL, d.Queue, errorMsg)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	result := d.downloadChunks(getResp.Body, file, startByte, totalSize)

	if result.Error != nil && !result.ShouldRetry {
		return result.Error
	}

	if result.Completed {
		if totalSize <= 0 {
			d.mutex.Lock()
			d.TotalSize = result.Downloaded
			d.Progress = 100.0
			d.mutex.Unlock()
		}
		logger.LogDownloadStatus(d.URL, "downloading", "completed", result.Downloaded, result.Downloaded)
		return nil
	}

	if result.Downloaded > (result.TotalSize*95/100) && supportsRanges {
		d.mutex.Lock()
		d.Downloaded = result.Downloaded
		d.mutex.Unlock()
		return nil
	}

	return fmt.Errorf("download incomplete: got %d of %d bytes", result.Downloaded, result.TotalSize)
}

// downloadChunks handles the actual data transfer
func (d *Download) downloadChunks(body io.Reader, file *os.File, startByte, totalSize int64) DownloadResult {
	// Setup rate limiting if needed
	var limiter *RateLimiter
	if d.MaxBandwidth > 0 {
		limiter = NewRateLimiter(d.MaxBandwidth * 1024) // Convert KB/s to bytes/s
		logger.LogDownloadPending(d.URL, d.Queue, fmt.Sprintf("Applying bandwidth limit of %d KB/s", d.MaxBandwidth))
	}

	// Track progress
	buffer := make([]byte, 32*1024)
	downloaded := startByte
	startTime := time.Now()
	lastUpdateTime := startTime
	lastBytes := downloaded

	// Start the download loop
	for {
		// Check if we should pause
		select {
		case <-d.pauseChan:
			logger.LogDownloadStatus(d.URL, "downloading", "paused", downloaded, totalSize)
			<-d.resumeChan
			startTime = time.Now()
			lastUpdateTime = startTime
			lastBytes = downloaded
			logger.LogDownloadStatus(d.URL, "paused", "downloading", downloaded, totalSize)
			continue

		case <-d.cancelChan:
			logger.LogDownloadStatus(d.URL, "downloading", "cancelled", downloaded, totalSize)
			return DownloadResult{
				Completed:   false,
				Downloaded:  downloaded,
				TotalSize:   totalSize,
				Error:       fmt.Errorf("download cancelled"),
				ShouldRetry: false,
			}

		default:
			// Proceed with download
		}

		// Read chunk
		var n int
		var err error
		if limiter != nil {
			n, err = limiter.Read(body, buffer)
		} else {
			n, err = body.Read(buffer)
		}

		if err != nil && err != io.EOF {
			return DownloadResult{
				Completed:   false,
				Downloaded:  downloaded,
				TotalSize:   totalSize,
				Error:       fmt.Errorf("error reading from response: %w", err),
				ShouldRetry: true,
			}
		}

		if n == 0 {
			break
		}

		// Write chunk
		if _, err := file.Write(buffer[:n]); err != nil {
			return DownloadResult{
				Completed:   false,
				Downloaded:  downloaded,
				TotalSize:   totalSize,
				Error:       fmt.Errorf("error writing to file: %w", err),
				ShouldRetry: true,
			}
		}

		downloaded += int64(n)

		// Update progress
		if totalSize > 0 {
			d.mutex.Lock()
			d.Progress = float64(downloaded) / float64(totalSize) * 100
			d.Downloaded = downloaded
			d.mutex.Unlock()
		}

		// Calculate speed and log progress
		now := time.Now()
		elapsed := now.Sub(lastUpdateTime)
		if elapsed >= time.Second {
			bytesPerSecond := int64(float64(downloaded-lastBytes) / elapsed.Seconds())
			d.mutex.Lock()
			d.Speed = bytesPerSecond
			d.mutex.Unlock()

			// Only calculate progress if we have a valid total size
			if totalSize > 0 {
				progressPercent := float64(downloaded) / float64(totalSize) * 100
				lastProgressPercent := float64(lastBytes) / float64(totalSize) * 100
				if (int(progressPercent/10) > int(lastProgressPercent/10)) || elapsed >= 30*time.Second {
					logger.LogDownloadStatus(d.URL, "downloading", "downloading", downloaded, totalSize)
				}
			}

			lastUpdateTime = now
			lastBytes = downloaded
		}
	}

	return DownloadResult{
		Completed:   downloaded >= totalSize || totalSize <= 0,
		Downloaded:  downloaded,
		TotalSize:   totalSize,
		Error:       nil,
		ShouldRetry: false,
	}
}

// New creates a new download instance
func New(url, targetPath, queue string, maxBandwidth int64, scheduledStartTime time.Time) *Download {
	download := &Download{
		URL:                url,
		TargetPath:         targetPath,
		Filename:           filepath.Base(targetPath),
		Queue:              queue,
		Status:             "pending",
		MaxBandwidth:       maxBandwidth,
		MaxRetries:         3,
		RetryDelay:         5 * time.Second,
		ScheduledStartTime: scheduledStartTime,
	}
	download.Initialize()
	return download
}

// StartDownload is a convenience function to create and start a download
func StartDownload(url, targetPath, queue string, maxBandwidth int64, scheduledStartTime time.Time) (*Download, error) {
	download := New(url, targetPath, queue, maxBandwidth, scheduledStartTime)
	go download.Start()
	return download, nil
}
