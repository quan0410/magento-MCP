package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"golang.org/x/image/draw"
)

// uploadProductImageFromURL downloads an image from a public URL and uploads it to Magento via /products/{sku}/media.
func uploadProductImageFromURL(ctx context.Context, client *magento.Client, sku string, imageURL string, isMain bool, position int) error {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil
	}

	// Optimize Unsplash image URLs: if missing width/quality, request web-optimized dimensions (1000px, 80% quality)
	// This reduces raw DSLR image size from ~4.5MB down to ~50KB, speeding up Magento PHP-FPM image processing by 70x
	// and completely eliminating 'context deadline exceeded' timeouts.
	if strings.Contains(imageURL, "images.unsplash.com") && !strings.Contains(imageURL, "w=") {
		if strings.Contains(imageURL, "?") {
			imageURL += "&auto=format&fit=crop&w=1000&q=80"
		} else {
			imageURL += "?auto=format&fit=crop&w=1000&q=80"
		}
	}

	downloadClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return fmt.Errorf("invalid image URL '%s': %w", imageURL, err)
	}
	req.Header.Set("User-Agent", "DatumBridge-Magento-MCP/1.0")

	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download image from '%s': %w", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("image URL '%s' returned HTTP status %d", imageURL, resp.StatusCode)
	}

	// Read image bytes, cap at 15MB
	limitReader := io.LimitReader(resp.Body, 15*1024*1024)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return fmt.Errorf("failed to read image data from '%s': %w", imageURL, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("image from '%s' is empty (0 bytes)", imageURL)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)

	// Automatically resize image if either width or height exceeds 1200px (e-commerce catalog standard)
	// Reduces 4MB-15MB DSLR raw photos down to ~100KB-150KB, eliminating PHP-FPM timeouts on Magento
	data, mimeType = resizeImageIfExceedsMaxDim(data, mimeType, 1200)

	if !strings.HasPrefix(mimeType, "image/") {
		// Fallback to extension check
		u, uErr := url.Parse(imageURL)
		if uErr == nil {
			ext := strings.ToLower(path.Ext(u.Path))
			switch ext {
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".png":
				mimeType = "image/png"
			case ".webp":
				mimeType = "image/webp"
			case ".gif":
				mimeType = "image/gif"
			}
		}
	}
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg"
	}

	// Derive filename
	fileName := path.Base(imageURL)
	if idx := strings.Index(fileName, "?"); idx != -1 {
		fileName = fileName[:idx]
	}
	if !strings.Contains(fileName, ".") {
		ext := ".jpg"
		if strings.Contains(mimeType, "png") {
			ext = ".png"
		} else if strings.Contains(mimeType, "webp") {
			ext = ".webp"
		}
		fileName = fmt.Sprintf("%s_%d%s", sku, position, ext)
	}

	b64Data := base64.StdEncoding.EncodeToString(data)

	types := []string{}
	if isMain {
		types = []string{"image", "small_image", "thumbnail"}
	}

	entry := map[string]interface{}{
		"media_type": "image",
		"label":      fmt.Sprintf("%s Image %d", sku, position),
		"position":   position,
		"disabled":   false,
		"types":      types,
		"content": map[string]interface{}{
			"base64_encoded_data": b64Data,
			"type":                mimeType,
			"name":                fileName,
		},
	}

	payload := map[string]interface{}{
		"entry": entry,
	}

	endpoint := fmt.Sprintf("/products/%s/media", url.PathEscape(sku))
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		_, _, lastErr = client.Request(ctx, http.MethodPost, endpoint, url.Values{}, payload)
		if lastErr == nil {
			return nil
		}
		if attempt < 2 {
			time.Sleep(1 * time.Second)
		}
	}
	return fmt.Errorf("Magento media upload failed for '%s': %w", imageURL, lastErr)
}

// uploadProductVideo uploads an external video link (YouTube / Vimeo) to Magento media gallery.
func uploadProductVideo(ctx context.Context, client *magento.Client, sku string, videoURL, videoTitle string, pos int) error {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return nil
	}
	if videoTitle == "" {
		videoTitle = fmt.Sprintf("%s Video", sku)
	}

	provider := "youtube"
	if strings.Contains(strings.ToLower(videoURL), "vimeo") {
		provider = "vimeo"
	}

	// 1x1 transparent PNG thumbnail placeholder
	transparentPNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

	entry := map[string]interface{}{
		"media_type": "external-video",
		"label":      videoTitle,
		"position":   pos,
		"disabled":   false,
		"types":      []string{},
		"extension_attributes": map[string]interface{}{
			"video_content": map[string]interface{}{
				"media_type":        "external-video",
				"video_provider":    provider,
				"video_url":         videoURL,
				"video_title":       videoTitle,
				"video_description": videoTitle,
				"video_metadata":    "",
			},
		},
		"content": map[string]interface{}{
			"base64_encoded_data": transparentPNG,
			"type":                "image/png",
			"name":                fmt.Sprintf("%s_video.png", sku),
		},
	}

	payload := map[string]interface{}{
		"entry": entry,
	}

	endpoint := fmt.Sprintf("/products/%s/media", url.PathEscape(sku))
	_, _, err := client.Request(ctx, http.MethodPost, endpoint, url.Values{}, payload)
	if err != nil {
		return fmt.Errorf("Magento video upload failed for '%s': %w", videoURL, err)
	}
	return nil
}

// uploadProductMedia handles uploading main image, additional gallery images, and external video for a product SKU.
func uploadProductMedia(ctx context.Context, client *magento.Client, sku string, mainImageURL, additionalImages, videoURL, videoTitle string) []string {
	var warnings []string
	pos := 1

	if mainImageURL != "" {
		if err := uploadProductImageFromURL(ctx, client, sku, mainImageURL, true, pos); err != nil {
			warnings = append(warnings, fmt.Sprintf("SKU '%s' main image warning: %v", sku, err))
		}
		pos++
	}

	if additionalImages != "" {
		urls := strings.Split(additionalImages, ",")
		for _, rawURL := range urls {
			u := strings.TrimSpace(rawURL)
			if u == "" {
				continue
			}
			time.Sleep(100 * time.Millisecond)
			if err := uploadProductImageFromURL(ctx, client, sku, u, false, pos); err != nil {
				warnings = append(warnings, fmt.Sprintf("SKU '%s' additional image warning: %v", sku, err))
			}
			pos++
		}
	}

	if videoURL != "" {
		time.Sleep(100 * time.Millisecond)
		if err := uploadProductVideo(ctx, client, sku, videoURL, videoTitle, pos); err != nil {
			warnings = append(warnings, fmt.Sprintf("SKU '%s' video warning: %v", sku, err))
		}
	}

	return warnings
}

// MediaTask represents a product media upload item to be processed.
type MediaTask struct {
	SKU              string `json:"sku"`
	MainImageURL     string `json:"image_url,omitempty"`
	AdditionalImages string `json:"additional_images,omitempty"`
	VideoURL         string `json:"video_url,omitempty"`
	VideoTitle       string `json:"video_title,omitempty"`
}

// MediaImportStatus represents the progress of asynchronous/deferred media uploads.
type MediaImportStatus struct {
	IsRunning       bool     `json:"is_running"`
	TotalQueued     int      `json:"total_queued"`
	Completed       int      `json:"completed"`
	Failed          int      `json:"failed"`
	Pending         int      `json:"pending"`
	RecentWarnings  []string `json:"recent_warnings"`
	LastStartedAt   string   `json:"last_started_at,omitempty"`
	LastCompletedAt string   `json:"last_completed_at,omitempty"`
}

type deferredMediaQueue struct {
	mu              sync.RWMutex
	tasks           []MediaTask
	client          *magento.Client
	running         bool
	total           int
	completed       int
	failed          int
	warnings        []string
	lastStartedAt   time.Time
	lastCompletedAt time.Time
}

var globalMediaQueue = &deferredMediaQueue{}

// Enqueue adds media tasks to the background processing queue.
func (q *deferredMediaQueue) Enqueue(client *magento.Client, tasks []MediaTask) int {
	if len(tasks) == 0 {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	q.client = client
	q.tasks = append(q.tasks, tasks...)
	q.total += len(tasks)

	if !q.running {
		q.running = true
		q.lastStartedAt = time.Now()
		go q.processLoop()
	}
	return len(tasks)
}

func (q *deferredMediaQueue) processLoop() {
	workerCount := 2
	taskChan := make(chan MediaTask)
	var wg sync.WaitGroup

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				warns := uploadProductMedia(ctx, q.client, task.SKU, task.MainImageURL, task.AdditionalImages, task.VideoURL, task.VideoTitle)
				cancel()

				q.mu.Lock()
				if len(warns) > 0 {
					q.failed++
					if len(q.warnings) < 100 {
						q.warnings = append(q.warnings, warns...)
					}
				} else {
					q.completed++
				}
				q.mu.Unlock()
			}
		}()
	}

	for {
		q.mu.Lock()
		if len(q.tasks) == 0 {
			q.mu.Unlock()
			break
		}
		task := q.tasks[0]
		q.tasks = q.tasks[1:]
		q.mu.Unlock()

		taskChan <- task
	}

	close(taskChan)
	wg.Wait()

	q.mu.Lock()
	q.running = false
	q.lastCompletedAt = time.Now()
	q.mu.Unlock()
}

// GetStatus returns the current media processing status.
func (q *deferredMediaQueue) GetStatus() MediaImportStatus {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pending := q.total - q.completed - q.failed
	if pending < 0 {
		pending = 0
	}

	var startedStr, completedStr string
	if !q.lastStartedAt.IsZero() {
		startedStr = q.lastStartedAt.Format(time.RFC3339)
	}
	if !q.lastCompletedAt.IsZero() {
		completedStr = q.lastCompletedAt.Format(time.RFC3339)
	}

	recentWarns := make([]string, len(q.warnings))
	copy(recentWarns, q.warnings)

	return MediaImportStatus{
		IsRunning:       q.running,
		TotalQueued:     q.total,
		Completed:       q.completed,
		Failed:          q.failed,
		Pending:         pending,
		RecentWarnings:  recentWarns,
		LastStartedAt:   startedStr,
		LastCompletedAt: completedStr,
	}
}

// handleMagentoGetMediaImportStatus returns the background media import queue status.
func handleMagentoGetMediaImportStatus(args json.RawMessage) map[string]interface{} {
	status := globalMediaQueue.GetStatus()
	resp := map[string]interface{}{
		"is_running":      status.IsRunning,
		"total_queued":    status.TotalQueued,
		"completed":       status.Completed,
		"failed":          status.Failed,
		"pending":         status.Pending,
		"last_started_at": status.LastStartedAt,
		"warnings":        status.RecentWarnings,
	}
	if status.LastCompletedAt != "" {
		resp["last_completed_at"] = status.LastCompletedAt
	}
	return resp
}

// resizeImageIfExceedsMaxDim inspects image dimensions and scales down to maxDim (e.g. 1200px)
// while preserving aspect ratio. This eliminates timeouts from raw camera images (4MB-15MB)
// and ensures consistent crisp catalog display on Magento.
func resizeImageIfExceedsMaxDim(data []byte, mimeType string, maxDim int) ([]byte, string) {
	if maxDim <= 0 {
		maxDim = 1200
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return data, mimeType // If config cannot be read, return data unchanged
	}

	if cfg.Width <= maxDim && cfg.Height <= maxDim {
		return data, mimeType
	}

	// Calculate new dimensions preserving aspect ratio
	newW, newH := cfg.Width, cfg.Height
	if newW > newH {
		newH = int(float64(newH) * float64(maxDim) / float64(newW))
		newW = maxDim
	} else {
		newW = int(float64(newW) * float64(maxDim) / float64(newH))
		newH = maxDim
	}
	if newW <= 0 {
		newW = 1
	}
	if newH <= 0 {
		newH = 1
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mimeType
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return data, mimeType
	}

	return buf.Bytes(), "image/jpeg"
}


