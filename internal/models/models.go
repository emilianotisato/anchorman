package models

import "time"

type Company struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Project struct {
	ID        int64
	Name      string
	CompanyID *int64 // nullable for orphans
	CreatedAt time.Time

	// Joined fields
	CompanyName string
}

type Repo struct {
	ID        int64
	Path      string
	ProjectID *int64 // nullable for orphans
	CreatedAt time.Time

	// Joined fields
	ProjectName string
	CompanyName string
}

type RawCommit struct {
	ID           int64
	RepoID       int64
	Hash         string
	Message      string
	Author       string
	Branch       string
	FilesChanged []string
	CommittedAt  time.Time
	Processed    bool
	CreatedAt    time.Time

	// Joined fields
	RepoPath string
}

type Task struct {
	ID             int64
	ProjectID      int64
	Description    string
	SourceCommits  []int64
	TaskDate       time.Time
	EstimatedHours float64 // 0.5 increments: 0.5, 1.0, 1.5, etc.
	CreatedAt      time.Time

	// Joined fields
	ProjectName string
}
