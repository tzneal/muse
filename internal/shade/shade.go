package shade

import (
	"context"
	"fmt"

	"github.com/ellistarn/shade/internal/source"
	"github.com/ellistarn/shade/internal/storage"
)

// UploadResult summarizes what happened during an upload sync.
type UploadResult struct {
	Total    int
	Uploaded int
	Skipped  int
	Warnings []string
}

// Shade holds the state needed for all operations.
type Shade struct {
	storage *storage.Client
	bucket  string
}

func New(ctx context.Context, bucket string) (*Shade, error) {
	client, err := storage.NewClient(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	return &Shade{storage: client, bucket: bucket}, nil
}

// Upload scans local sources, diffs against S3, and uploads changed sessions.
func (s *Shade) Upload(ctx context.Context) (*UploadResult, error) {
	existing, err := s.storage.ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote sessions: %w", err)
	}
	remote := map[string]storage.SessionEntry{}
	for _, e := range existing {
		remote[e.Key] = e
	}

	var local []source.Session
	var warnings []string
	if sessions, err := source.OpenCodeSessions(); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to read OpenCode sessions: %v", err))
	} else {
		local = append(local, sessions...)
	}
	if sessions, err := source.ClaudeCodeSessions(); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to read Claude Code sessions: %v", err))
	} else {
		local = append(local, sessions...)
	}

	var uploaded, skipped int
	for i := range local {
		sess := &local[i]
		key := fmt.Sprintf("sessions/%s/%s.json", sess.Source, sess.SessionID)
		if entry, exists := remote[key]; exists {
			if !sess.UpdatedAt.After(entry.LastModified) {
				skipped++
				continue
			}
		}
		if err := s.storage.PutSession(ctx, sess); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to upload %s: %v", sess.SessionID, err))
			continue
		}
		uploaded++
	}
	return &UploadResult{
		Total:    len(local),
		Uploaded: uploaded,
		Skipped:  skipped,
		Warnings: warnings,
	}, nil
}

// Ls returns all session entries from S3.
func (s *Shade) Ls(ctx context.Context) ([]storage.SessionEntry, error) {
	entries, err := s.storage.ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	return entries, nil
}

// Show retrieves a specific session from S3. If source is empty, both sources
// are tried.
func (s *Shade) Show(ctx context.Context, sessionID string, src string) (*source.Session, error) {
	sources := []string{src}
	if src == "" {
		sources = []string{"opencode", "claude-code"}
	}
	for _, source := range sources {
		session, err := s.storage.GetSession(ctx, source, sessionID)
		if err != nil {
			continue
		}
		return session, nil
	}
	return nil, fmt.Errorf("session %s not found", sessionID)
}
