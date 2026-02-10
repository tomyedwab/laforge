package main

import (
	"bufio"
	"encoding/json"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tom/laforge/viewer/internal/db"
	"github.com/tom/laforge/viewer/internal/render"
	"github.com/tom/laforge/viewer/internal/watcher"
)

var (
	database  *db.DB
	templates *template.Template
	logsDir   string
)

func main() {
	// Configuration from environment or defaults
	logsDir = getEnv("LOGS_DIR", "/logs")
	dbPath := getEnv("DB_PATH", "/data/viewer.db")
	port := getEnv("PORT", "8082")

	// Initialize database
	var err error
	database, err = db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	slog.Info("database initialized", "path", dbPath)

	// Initialize file watcher
	w, err := watcher.New(logsDir, database)
	if err != nil {
		log.Fatalf("Failed to initialize watcher: %v", err)
	}
	defer w.Close()

	w.Start()
	slog.Info("file watcher started", "dir", logsDir)

	// Load templates
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// HTTP routes
	http.HandleFunc("/", handleJobsList)
	http.HandleFunc("/jobs/", handleJobView)
	http.HandleFunc("/api/jobs", handleAPIJobs)
	http.HandleFunc("/api/jobs/", handleAPIJobContent)

	// Static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start server
	addr := ":" + port
	slog.Info("starting HTTP server", "address", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// handleJobsList renders the jobs list page
func handleJobsList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	jobs, err := database.GetAllJobs()
	if err != nil {
		slog.Error("failed to get jobs", "error", err)
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}

	data := struct {
		Jobs []db.Job
	}{
		Jobs: jobs,
	}

	var contentBuf strings.Builder
	if err := templates.ExecuteTemplate(&contentBuf, "jobs.html", data); err != nil {
		slog.Error("failed to render jobs template", "error", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

	layoutData := struct {
		Title   string
		Content template.HTML
	}{
		Title:   "Jobs",
		Content: template.HTML(contentBuf.String()),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "layout.html", layoutData); err != nil {
		slog.Error("failed to render layout", "error", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

// handleJobView renders the individual job view page
func handleJobView(w http.ResponseWriter, r *http.Request) {
	// Extract job name from URL
	jobName := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if jobName == "" {
		http.NotFound(w, r)
		return
	}

	job, err := database.GetJobByName(jobName)
	if err != nil {
		slog.Error("failed to get job", "job_name", jobName, "error", err)
		http.Error(w, "Failed to fetch job", http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.NotFound(w, r)
		return
	}

	// Read and render log file
	logHTML, lineCount, err := renderLogFile(job.LogFile, 0)
	if err != nil {
		slog.Error("failed to render log file", "file", job.LogFile, "error", err)
		logHTML = "<p>Error loading log file</p>"
	}

	data := struct {
		Job      *db.Job
		LogHTML  template.HTML
		LogLines int
	}{
		Job:      job,
		LogHTML:  template.HTML(logHTML),
		LogLines: lineCount,
	}

	var contentBuf strings.Builder
	if err := templates.ExecuteTemplate(&contentBuf, "job.html", data); err != nil {
		slog.Error("failed to render job template", "error", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

	layoutData := struct {
		Title   string
		Content template.HTML
	}{
		Title:   job.JobName,
		Content: template.HTML(contentBuf.String()),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "layout.html", layoutData); err != nil {
		slog.Error("failed to render layout", "error", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

// handleAPIJobs returns jobs as JSON
func handleAPIJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := database.GetAllJobs()
	if err != nil {
		slog.Error("failed to get jobs", "error", err)
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// handleAPIJobContent returns log content from a given offset
func handleAPIJobContent(w http.ResponseWriter, r *http.Request) {
	// Extract job name from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/jobs/"), "/")
	if len(pathParts) < 2 || pathParts[1] != "content" {
		http.NotFound(w, r)
		return
	}

	jobName := pathParts[0]
	if jobName == "" {
		http.NotFound(w, r)
		return
	}

	// Get offset from query params
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			http.Error(w, "Invalid offset", http.StatusBadRequest)
			return
		}
	}

	// Get job from database
	job, err := database.GetJobByName(jobName)
	if err != nil {
		slog.Error("failed to get job", "job_name", jobName, "error", err)
		http.Error(w, "Failed to fetch job", http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.NotFound(w, r)
		return
	}

	// Render log file from offset
	logHTML, newOffset, err := renderLogFile(job.LogFile, offset)
	if err != nil {
		slog.Error("failed to render log file", "file", job.LogFile, "error", err)
		http.Error(w, "Failed to read log file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("X-Log-Offset", strconv.Itoa(newOffset))
	w.Write([]byte(logHTML))
}

// renderLogFile reads and renders a log file, optionally starting from an offset
func renderLogFile(logFile string, offset int) (string, int, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	var output strings.Builder
	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines (e.g., tool results with file contents)
	scanner.Buffer(make([]byte, 0), 10*1024*1024) // 10MB max
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Skip lines before offset
		if lineNum < offset {
			lineNum++
			continue
		}

		// Render the line
		html := render.RenderMessage(line)
		output.WriteString(html)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", lineNum, err
	}

	return output.String(), lineNum, nil
}
