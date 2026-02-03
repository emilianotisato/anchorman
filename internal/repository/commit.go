package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/emilianohg/anchorman/internal/models"
)

type CommitRepo struct {
	db *sql.DB
}

func NewCommitRepo(db *sql.DB) *CommitRepo {
	return &CommitRepo{db: db}
}

func (r *CommitRepo) Create(repoID int64, hash, message, author, branch string, filesChanged []string, committedAt time.Time) (*models.RawCommit, error) {
	filesJSON, err := json.Marshal(filesChanged)
	if err != nil {
		return nil, err
	}

	result, err := r.db.Exec(`
		INSERT INTO raw_commits (repo_id, hash, message, author, branch, files_changed, committed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, repoID, hash, message, author, branch, string(filesJSON), committedAt)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *CommitRepo) GetByID(id int64) (*models.RawCommit, error) {
	var c models.RawCommit
	var filesJSON string

	err := r.db.QueryRow(`
		SELECT rc.id, rc.repo_id, rc.hash, rc.message, rc.author, rc.branch,
		       rc.files_changed, rc.committed_at, rc.processed, rc.created_at, r.path
		FROM raw_commits rc
		JOIN repos r ON r.id = rc.repo_id
		WHERE rc.id = ?
	`, id).Scan(
		&c.ID, &c.RepoID, &c.Hash, &c.Message, &c.Author, &c.Branch,
		&filesJSON, &c.CommittedAt, &c.Processed, &c.CreatedAt, &c.RepoPath,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(filesJSON), &c.FilesChanged); err != nil {
		return nil, err
	}

	return &c, nil
}

func (r *CommitRepo) GetByRepoAndHash(repoID int64, hash string) (*models.RawCommit, error) {
	var c models.RawCommit
	var filesJSON string

	err := r.db.QueryRow(`
		SELECT rc.id, rc.repo_id, rc.hash, rc.message, rc.author, rc.branch,
		       rc.files_changed, rc.committed_at, rc.processed, rc.created_at, r.path
		FROM raw_commits rc
		JOIN repos r ON r.id = rc.repo_id
		WHERE rc.repo_id = ? AND rc.hash = ?
	`, repoID, hash).Scan(
		&c.ID, &c.RepoID, &c.Hash, &c.Message, &c.Author, &c.Branch,
		&filesJSON, &c.CommittedAt, &c.Processed, &c.CreatedAt, &c.RepoPath,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(filesJSON), &c.FilesChanged); err != nil {
		return nil, err
	}

	return &c, nil
}

func (r *CommitRepo) GetUnprocessed() ([]models.RawCommit, error) {
	return r.getCommitsWithFilter("WHERE rc.processed = 0", nil)
}

func (r *CommitRepo) GetUnprocessedInDateRange(from, to time.Time) ([]models.RawCommit, error) {
	return r.getCommitsWithFilter(
		"WHERE rc.processed = 0 AND rc.committed_at >= ? AND rc.committed_at <= ?",
		[]interface{}{from, to},
	)
}

func (r *CommitRepo) GetUnprocessedByProjectID(projectID int64) ([]models.RawCommit, error) {
	return r.getCommitsWithFilter(
		"JOIN projects p ON p.id = re.project_id WHERE rc.processed = 0 AND p.id = ?",
		[]interface{}{projectID},
	)
}

func (r *CommitRepo) getCommitsWithFilter(filter string, args []interface{}) ([]models.RawCommit, error) {
	query := `
		SELECT rc.id, rc.repo_id, rc.hash, rc.message, rc.author, rc.branch,
		       rc.files_changed, rc.committed_at, rc.processed, rc.created_at, re.path
		FROM raw_commits rc
		JOIN repos re ON re.id = rc.repo_id
		` + filter + `
		ORDER BY rc.committed_at ASC
	`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commits []models.RawCommit
	for rows.Next() {
		var c models.RawCommit
		var filesJSON string

		if err := rows.Scan(
			&c.ID, &c.RepoID, &c.Hash, &c.Message, &c.Author, &c.Branch,
			&filesJSON, &c.CommittedAt, &c.Processed, &c.CreatedAt, &c.RepoPath,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(filesJSON), &c.FilesChanged); err != nil {
			return nil, err
		}

		commits = append(commits, c)
	}
	return commits, rows.Err()
}

func (r *CommitRepo) MarkProcessed(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Build placeholders
	placeholders := ""
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	_, err := r.db.Exec(
		"UPDATE raw_commits SET processed = 1 WHERE id IN ("+placeholders+")",
		args...,
	)
	return err
}

func (r *CommitRepo) CountUnprocessed() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM raw_commits WHERE processed = 0").Scan(&count)
	return count, err
}

func (r *CommitRepo) GetLastProcessedTime() (*time.Time, error) {
	var lastTime sql.NullTime
	err := r.db.QueryRow(`
		SELECT MAX(created_at) FROM tasks
	`).Scan(&lastTime)

	if err != nil {
		return nil, err
	}

	if !lastTime.Valid {
		return nil, nil
	}

	return &lastTime.Time, nil
}
