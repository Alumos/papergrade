package app

import (
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/vision"
	"time"
)

type Class struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	StudentCount int       `json:"student_count"`
	CreatedAt    time.Time `json:"created_at"`
}
type Student struct {
	ID      string `json:"id"`
	ClassID string `json:"class_id"`
	Number  string `json:"number"`
	Name    string `json:"name"`
}
type Sheet struct {
	StudentID     string `json:"student_id"`
	StudentNumber string `json:"student_number"`
	StudentName   string `json:"student_name"`
	Token         string `json:"token"`
}
type Exam struct {
	StudentNumberDigits int                `json:"student_number_digits"`
	ID                  string             `json:"id"`
	Title               string             `json:"title"`
	Subject             string             `json:"subject"`
	ClassID             string             `json:"class_id"`
	Status              string             `json:"status"`
	Revision            int                `json:"revision"`
	Questions           []grading.Question `json:"questions"`
	ReleaseID           string             `json:"release_id,omitempty"`
	GradingVersion      string             `json:"grading_version,omitempty"`
	Template            *vision.Manifest   `json:"template,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	StudentCount        int                `json:"student_count"`
	ReceivedCount       int                `json:"received_count"`
	ReviewCount         int                `json:"review_count"`
	Sheets              []Sheet            `json:"sheets,omitempty"`
}
type Capture struct {
	StudentNumber  string           `json:"student_number,omitempty"`
	IdentityState  string           `json:"identity_state,omitempty"`
	Revision       int              `json:"revision"`
	ID             string           `json:"capture_id"`
	ClientID       string           `json:"client_capture_id"`
	ExamID         string           `json:"exam_id"`
	Status         string           `json:"status"`
	StudentDisplay string           `json:"student_display,omitempty"`
	SubmissionID   string           `json:"submission_id,omitempty"`
	Score          int              `json:"score_units"`
	Max            int              `json:"max_score_units"`
	ReviewCount    int              `json:"review_count"`
	Message        string           `json:"message,omitempty"`
	Deduplicated   bool             `json:"deduplicated,omitempty"`
	Answers        []grading.Answer `json:"answers,omitempty"`
	Hash           string           `json:"hash,omitempty"`
	Path           string           `json:"path,omitempty"`
	Candidate      string           `json:"candidate,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}
type Result struct {
	ID             string           `json:"id"`
	ExamID         string           `json:"exam_id"`
	StudentID      string           `json:"student_id"`
	StudentNumber  string           `json:"student_number"`
	StudentName    string           `json:"student_name"`
	Status         string           `json:"status"`
	Score          int              `json:"score_units"`
	Max            int              `json:"max_score_units"`
	Answers        []grading.Answer `json:"answers"`
	Revision       int              `json:"revision"`
	CaptureID      string           `json:"capture_id"`
	Engine         string           `json:"engine"`
	ReleaseID      string           `json:"release_id"`
	GradingVersion string           `json:"grading_version,omitempty"`
}
type Session struct {
	Expires time.Time `json:"expires"`
}
type Account struct {
	Username     string
	PasswordHash []byte
}
type PageError struct {
	PageNumber int    `json:"page_number"`
	Message    string `json:"message"`
}
type DocumentPage struct {
	ID          string `json:"id"`
	DocumentID  string `json:"document_id"`
	PageNumber  int    `json:"page_number"`
	TotalPages  int    `json:"total_pages"`
	Status      string `json:"status"`
	ContentType string `json:"content_type"`
	StoragePath string `json:"storage_path,omitempty"`
	Hash        string `json:"hash,omitempty"`
	Text        string `json:"text,omitempty"`
	Error       string `json:"error,omitempty"`
}
type DocumentTask struct {
	DocumentID string `json:"document_id"`
	PageNumber int    `json:"page_number"`
	Generation int    `json:"generation"`
	Phase      string `json:"phase"`
	Status     string `json:"status"`
}
type Document struct {
	ContentType     string      `json:"content_type"`
	TotalPages      int         `json:"total_pages"`
	PreparedPages   int         `json:"prepared_pages"`
	CompletedPages  int         `json:"completed_pages"`
	CurrentPage     int         `json:"current_page"`
	Progress        int         `json:"progress"`
	ParseGeneration int         `json:"parse_generation"`
	PageErrors      []PageError `json:"page_errors"`
	ID              string      `json:"id"`
	ExamID          string      `json:"exam_id"`
	Status          string      `json:"status"`
	Filename        string      `json:"filename"`
	Size            int         `json:"size"`
	Path            string      `json:"-"`
	StoragePath     string      `json:"storage_path,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	Artifact        any         `json:"artifact,omitempty"`
}
