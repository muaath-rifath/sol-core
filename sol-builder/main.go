package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type BuildJobPayload struct {
	JobID       string `json:"job_id"`
	TemplateID  string `json:"template_id"`
	TargetBoard string `json:"target_board"`
}

func main() {
	redisURL := envOrDefault("REDIS_URL", "redis://localhost:6379/0")
	apiURL := envOrDefault("SOL_API_URL", "http://sol-core:8080")
	srcDir := envOrDefault("FIRMWARE_SRC_DIR", "/firmware")

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("failed to parse redis url: %v", err)
	}
	rdb := redis.NewClient(opt)

	log.Printf("Worker started. Listening on firmware_build_queue...")

	for {
		// BLPop blocks until a job is available
		res, err := rdb.BLPop(context.Background(), 0, "firmware_build_queue").Result()
		if err != nil {
			log.Printf("redis error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var payload BuildJobPayload
		if err := json.Unmarshal([]byte(res[1]), &payload); err != nil {
			log.Printf("failed to unmarshal payload: %v", err)
			continue
		}

		log.Printf("Starting build job %s (Template: %s, Board: %s)", payload.JobID, payload.TemplateID, payload.TargetBoard)
		runBuild(payload, apiURL, srcDir)
	}
}

func runBuild(payload BuildJobPayload, apiURL, srcDir string) {
	ctx := context.Background()

	// 1. Update status to building
	updateStatus(apiURL, payload.JobID, "building", "", "")

	// 2. Set target
	if err := execCommand(ctx, apiURL, payload.JobID, srcDir, "idf.py", "set-target", payload.TargetBoard); err != nil {
		updateStatus(apiURL, payload.JobID, "failed", "", fmt.Sprintf("\nFailed to set target: %v", err))
		return
	}

	// 3. Build
	if err := execCommand(ctx, apiURL, payload.JobID, srcDir, "idf.py", "build"); err != nil {
		updateStatus(apiURL, payload.JobID, "failed", "", fmt.Sprintf("\nBuild failed: %v", err))
		return
	}

	// 4. Ingest binaries
	versionID, err := ingestBinaries(apiURL, payload.TemplateID, payload.TargetBoard, payload.JobID, srcDir)
	if err != nil {
		updateStatus(apiURL, payload.JobID, "failed", "", fmt.Sprintf("\nIngestion failed: %v", err))
		return
	}

	// 5. Success!
	updateStatus(apiURL, payload.JobID, "success", versionID, "\nBuild complete!")
}

func execCommand(ctx context.Context, apiURL, jobID, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	// Capture and stream logs
	pipeR, pipeW := io.Pipe()
	cmd.Stdout = pipeW
	cmd.Stderr = pipeW

	done := make(chan struct{})
	go func() {
		scanner := io.TeeReader(pipeR, os.Stdout)
		buf := make([]byte, 1024)
		for {
			n, err := scanner.Read(buf)
			if n > 0 {
				appendLogs(apiURL, jobID, string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	err := cmd.Run()
	pipeW.Close()
	<-done
	return err
}

func updateStatus(apiURL, jobID, status, versionID, logs string) {
	body, _ := json.Marshal(map[string]string{
		"status":              status,
		"firmware_version_id": versionID,
		"logs":                logs,
	})
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/internal/firmware/builds/%s", apiURL, jobID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	_, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to update status: %v", err)
	}
}

func appendLogs(apiURL, jobID, logs string) {
	body, _ := json.Marshal(map[string]string{"logs": logs})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/internal/firmware/builds/%s/logs", apiURL, jobID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	_, _ = http.DefaultClient.Do(req)
}

func ingestBinaries(apiURL, templateID, targetBoard, jobID, srcDir string) (string, error) {
	buildDir := filepath.Join(srcDir, "build")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	files := map[string]string{
		"bootloader":      filepath.Join(buildDir, "bootloader", "bootloader.bin"),
		"partition_table": filepath.Join(buildDir, "partition_table", "partition-table.bin"),
		"app":             filepath.Join(buildDir, "my_led.bin"),
	}

	for field, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()
		part, _ := writer.CreateFormFile(field, filepath.Base(path))
		io.Copy(part, file)
	}

	writer.WriteField("template_id", templateID)
	writer.WriteField("version", buildVersionTag(templateID, targetBoard, jobID, srcDir))
	writer.Close()

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/firmware/upload", apiURL), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	internalToken := os.Getenv("INTERNAL_SERVICE_TOKEN")
	if internalToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", internalToken))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %s", string(b))
	}

	var res struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.ID, nil
}

func buildVersionTag(templateID, targetBoard, jobID, srcDir string) string {
	source := strings.TrimSpace(os.Getenv("FIRMWARE_SOURCE_TAG"))
	if source == "" {
		source = gitShortSHA(srcDir)
	}
	if source == "" {
		source = projectVersion(srcDir)
	}
	if source == "" {
		if len(jobID) > 8 {
			source = jobID[:8]
		} else {
			source = jobID
		}
	}

	return fmt.Sprintf("%s-%s-%s-%s", sanitizeTagPart(templateID), sanitizeTagPart(targetBoard), time.Now().UTC().Format("20060102T150405Z"), sanitizeTagPart(source))
}

func gitShortSHA(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--short=8", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func projectVersion(srcDir string) string {
	data, err := os.ReadFile(filepath.Join(srcDir, "build", "project_description.json"))
	if err != nil {
		return ""
	}
	var desc struct {
		ProjectVersion string `json:"project_version"`
	}
	if err := json.Unmarshal(data, &desc); err != nil {
		return ""
	}
	return strings.TrimSpace(desc.ProjectVersion)
}

func sanitizeTagPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
