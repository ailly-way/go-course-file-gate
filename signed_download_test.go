package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type gatewaySpy struct {
	verifyCalls  int
	presignCalls int
	expires      int
}

func (g *gatewaySpy) VerifySession(context.Context, string) error {
	g.verifyCalls++
	return nil
}

func (g *gatewaySpy) PresignDownload(_ context.Context, _, _ string, expires int, _, _ string) (string, error) {
	g.presignCalls++
	g.expires = expires
	return "https://download.example/course.pdf", nil
}

func TestDownloadDecision(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		deadline     time.Time
		wantErr      error
		wantExpires  int
		wantPresigns int
	}{
		{name: "deadline limits link", deadline: now.Add(90 * time.Second), wantExpires: 90, wantPresigns: 1},
		{name: "link lifetime caps long enrollment", deadline: now.Add(24 * time.Hour), wantExpires: 600, wantPresigns: 1},
		{name: "past deadline denies download", deadline: now.Add(-time.Second), wantErr: errDeadlinePassed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &gatewaySpy{}
			service := DownloadService{Gateway: gateway, Bucket: "private-course-files", Now: func() time.Time { return now }, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			_, err := service.Issue(context.Background(), DownloadRequest{
				SessionID: "session-17", LearnerID: "learner-42", CourseID: "go-101", ObjectKey: "go-101/module-2.pdf", Deadline: tt.deadline,
			})
			if err != tt.wantErr {
				t.Fatalf("Issue() error = %v, want %v", err, tt.wantErr)
			}
			if gateway.verifyCalls != 1 {
				t.Fatalf("VerifySession calls = %d, want 1", gateway.verifyCalls)
			}
			if gateway.presignCalls != tt.wantPresigns {
				t.Fatalf("PresignDownload calls = %d, want %d", gateway.presignCalls, tt.wantPresigns)
			}
			if gateway.expires != tt.wantExpires {
				t.Fatalf("expires_seconds = %d, want %d", gateway.expires, tt.wantExpires)
			}
		})
	}
}
