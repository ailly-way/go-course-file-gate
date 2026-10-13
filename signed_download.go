package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
)

const maxLinkLifetime = 10 * time.Minute

var errDeadlinePassed = errors.New("course access deadline has passed")

type DownloadGateway interface {
	VerifySession(context.Context, string) error
	PresignDownload(context.Context, string, string, int, string, string) (string, error)
}

type DownloadRequest struct {
	SessionID string    `json:"session_id"`
	LearnerID string    `json:"learner_id"`
	CourseID  string    `json:"course_id"`
	ObjectKey string    `json:"object_key"`
	Deadline  time.Time `json:"deadline"`
}

type DownloadResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type DownloadService struct {
	Gateway DownloadGateway
	Bucket  string
	Now     func() time.Time
	Log     *slog.Logger
}

func (s DownloadService) Issue(ctx context.Context, in DownloadRequest) (DownloadResponse, error) {
	if in.SessionID == "" || in.LearnerID == "" || in.CourseID == "" || in.ObjectKey == "" || in.Deadline.IsZero() {
		return DownloadResponse{}, errors.New("session_id, learner_id, course_id, object_key, and deadline are required")
	}
	if err := s.Gateway.VerifySession(ctx, in.SessionID); err != nil {
		return DownloadResponse{}, err
	}
	now := s.Now().UTC()
	if !in.Deadline.After(now) {
		return DownloadResponse{}, errDeadlinePassed
	}
	expiresAt := now.Add(maxLinkLifetime)
	if in.Deadline.Before(expiresAt) {
		expiresAt = in.Deadline
	}
	expiresSeconds := int(expiresAt.Sub(now) / time.Second)
	disposition := `attachment; filename="` + strings.ReplaceAll(path.Base(in.ObjectKey), `"`, "") + `"`
	idempotencyKey := requestKey(in, expiresAt)
	downloadURL, err := s.Gateway.PresignDownload(ctx, s.Bucket, in.ObjectKey, expiresSeconds, disposition, idempotencyKey)
	if err != nil {
		return DownloadResponse{}, err
	}
	s.Log.Info("course download issued", "course_id", in.CourseID, "learner_id", in.LearnerID, "object_key", in.ObjectKey, "expires_at", expiresAt)
	return DownloadResponse{DownloadURL: downloadURL, ExpiresAt: expiresAt}, nil
}

func requestKey(in DownloadRequest, expiresAt time.Time) string {
	sum := sha256.Sum256([]byte(in.SessionID + "\x00" + in.CourseID + "\x00" + in.ObjectKey + "\x00" + expiresAt.Format(time.RFC3339)))
	return hex.EncodeToString(sum[:])
}

func (s DownloadService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	defer r.Body.Close()
	var in DownloadRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	out, err := s.Issue(r.Context(), in)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errDeadlinePassed) {
			status = http.StatusForbidden
		} else {
			var apiErr *InfraiError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				status = apiErr.Status
			}
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
