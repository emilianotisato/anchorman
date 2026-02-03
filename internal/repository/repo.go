package repository

import (
	"database/sql"

	"github.com/emilianohg/anchorman/internal/models"
)

type RepoRepo struct {
	db *sql.DB
}

func NewRepoRepo(db *sql.DB) *RepoRepo {
	return &RepoRepo{db: db}
}

func (r *RepoRepo) Create(path string, projectID *int64) (*models.Repo, error) {
	result, err := r.db.Exec(
		"INSERT INTO repos (path, project_id) VALUES (?, ?)",
		path, projectID,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *RepoRepo) GetOrCreate(path string) (*models.Repo, error) {
	repo, err := r.GetByPath(path)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}

	// Create as orphan
	return r.Create(path, nil)
}

func (r *RepoRepo) GetByID(id int64) (*models.Repo, error) {
	var repo models.Repo
	var projectID sql.NullInt64
	var projectName sql.NullString
	var companyName sql.NullString

	err := r.db.QueryRow(`
		SELECT r.id, r.path, r.project_id, r.created_at, p.name, c.name
		FROM repos r
		LEFT JOIN projects p ON p.id = r.project_id
		LEFT JOIN companies c ON c.id = p.company_id
		WHERE r.id = ?
	`, id).Scan(&repo.ID, &repo.Path, &projectID, &repo.CreatedAt, &projectName, &companyName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if projectID.Valid {
		repo.ProjectID = &projectID.Int64
	}
	repo.ProjectName = projectName.String
	repo.CompanyName = companyName.String

	return &repo, nil
}

func (r *RepoRepo) GetByPath(path string) (*models.Repo, error) {
	var repo models.Repo
	var projectID sql.NullInt64
	var projectName sql.NullString
	var companyName sql.NullString

	err := r.db.QueryRow(`
		SELECT r.id, r.path, r.project_id, r.created_at, p.name, c.name
		FROM repos r
		LEFT JOIN projects p ON p.id = r.project_id
		LEFT JOIN companies c ON c.id = p.company_id
		WHERE r.path = ?
	`, path).Scan(&repo.ID, &repo.Path, &projectID, &repo.CreatedAt, &projectName, &companyName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if projectID.Valid {
		repo.ProjectID = &projectID.Int64
	}
	repo.ProjectName = projectName.String
	repo.CompanyName = companyName.String

	return &repo, nil
}

func (r *RepoRepo) GetAll() ([]models.Repo, error) {
	rows, err := r.db.Query(`
		SELECT r.id, r.path, r.project_id, r.created_at, p.name, c.name
		FROM repos r
		LEFT JOIN projects p ON p.id = r.project_id
		LEFT JOIN companies c ON c.id = p.company_id
		ORDER BY c.name, p.name, r.path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.Repo
	for rows.Next() {
		var repo models.Repo
		var projectID sql.NullInt64
		var projectName sql.NullString
		var companyName sql.NullString

		if err := rows.Scan(&repo.ID, &repo.Path, &projectID, &repo.CreatedAt, &projectName, &companyName); err != nil {
			return nil, err
		}

		if projectID.Valid {
			repo.ProjectID = &projectID.Int64
		}
		repo.ProjectName = projectName.String
		repo.CompanyName = companyName.String

		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (r *RepoRepo) GetOrphans() ([]models.Repo, error) {
	rows, err := r.db.Query(`
		SELECT id, path, project_id, created_at
		FROM repos
		WHERE project_id IS NULL
		ORDER BY path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.Repo
	for rows.Next() {
		var repo models.Repo
		var projectID sql.NullInt64

		if err := rows.Scan(&repo.ID, &repo.Path, &projectID, &repo.CreatedAt); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (r *RepoRepo) GetByProjectID(projectID int64) ([]models.Repo, error) {
	rows, err := r.db.Query(`
		SELECT r.id, r.path, r.project_id, r.created_at, p.name, c.name
		FROM repos r
		LEFT JOIN projects p ON p.id = r.project_id
		LEFT JOIN companies c ON c.id = p.company_id
		WHERE r.project_id = ?
		ORDER BY r.path
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.Repo
	for rows.Next() {
		var repo models.Repo
		var projID sql.NullInt64
		var projectName sql.NullString
		var companyName sql.NullString

		if err := rows.Scan(&repo.ID, &repo.Path, &projID, &repo.CreatedAt, &projectName, &companyName); err != nil {
			return nil, err
		}

		if projID.Valid {
			repo.ProjectID = &projID.Int64
		}
		repo.ProjectName = projectName.String
		repo.CompanyName = companyName.String

		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (r *RepoRepo) SetProject(id int64, projectID *int64) error {
	_, err := r.db.Exec("UPDATE repos SET project_id = ? WHERE id = ?", projectID, id)
	return err
}

func (r *RepoRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM repos WHERE id = ?", id)
	return err
}

type RepoWithStats struct {
	models.Repo
	CommitCount           int
	UnprocessedCommitCount int
}

func (r *RepoRepo) GetAllWithStats() ([]RepoWithStats, error) {
	query := `
		SELECT
			r.id, r.path, r.project_id, r.created_at, p.name, c.name,
			COUNT(rc.id) as commit_count,
			SUM(CASE WHEN rc.processed = 0 THEN 1 ELSE 0 END) as unprocessed_count
		FROM repos r
		LEFT JOIN projects p ON p.id = r.project_id
		LEFT JOIN companies c ON c.id = p.company_id
		LEFT JOIN raw_commits rc ON rc.repo_id = r.id
		GROUP BY r.id
		ORDER BY c.name, p.name, r.path
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []RepoWithStats
	for rows.Next() {
		var repo RepoWithStats
		var projectID sql.NullInt64
		var projectName sql.NullString
		var companyName sql.NullString
		var unprocessedCount sql.NullInt64

		if err := rows.Scan(
			&repo.ID, &repo.Path, &projectID, &repo.CreatedAt, &projectName, &companyName,
			&repo.CommitCount, &unprocessedCount,
		); err != nil {
			return nil, err
		}

		if projectID.Valid {
			repo.ProjectID = &projectID.Int64
		}
		repo.ProjectName = projectName.String
		repo.CompanyName = companyName.String
		repo.UnprocessedCommitCount = int(unprocessedCount.Int64)

		repos = append(repos, repo)
	}
	return repos, rows.Err()
}
