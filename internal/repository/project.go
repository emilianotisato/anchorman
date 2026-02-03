package repository

import (
	"database/sql"

	"github.com/emilianohg/anchorman/internal/models"
)

type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(name string, companyID *int64) (*models.Project, error) {
	result, err := r.db.Exec(
		"INSERT INTO projects (name, company_id) VALUES (?, ?)",
		name, companyID,
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

func (r *ProjectRepo) GetByID(id int64) (*models.Project, error) {
	var p models.Project
	var companyID sql.NullInt64
	var companyName sql.NullString

	err := r.db.QueryRow(`
		SELECT p.id, p.name, p.company_id, p.created_at, c.name
		FROM projects p
		LEFT JOIN companies c ON c.id = p.company_id
		WHERE p.id = ?
	`, id).Scan(&p.ID, &p.Name, &companyID, &p.CreatedAt, &companyName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if companyID.Valid {
		p.CompanyID = &companyID.Int64
	}
	p.CompanyName = companyName.String

	return &p, nil
}

func (r *ProjectRepo) GetAll() ([]models.Project, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.name, p.company_id, p.created_at, c.name
		FROM projects p
		LEFT JOIN companies c ON c.id = p.company_id
		ORDER BY c.name, p.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var companyID sql.NullInt64
		var companyName sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &companyID, &p.CreatedAt, &companyName); err != nil {
			return nil, err
		}

		if companyID.Valid {
			p.CompanyID = &companyID.Int64
		}
		p.CompanyName = companyName.String

		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepo) GetByCompanyID(companyID int64) ([]models.Project, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.name, p.company_id, p.created_at, c.name
		FROM projects p
		LEFT JOIN companies c ON c.id = p.company_id
		WHERE p.company_id = ?
		ORDER BY p.name
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var companyID sql.NullInt64
		var companyName sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &companyID, &p.CreatedAt, &companyName); err != nil {
			return nil, err
		}

		if companyID.Valid {
			p.CompanyID = &companyID.Int64
		}
		p.CompanyName = companyName.String

		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepo) GetOrphans() ([]models.Project, error) {
	rows, err := r.db.Query(`
		SELECT id, name, company_id, created_at
		FROM projects
		WHERE company_id IS NULL
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var companyID sql.NullInt64

		if err := rows.Scan(&p.ID, &p.Name, &companyID, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepo) Update(id int64, name string) error {
	_, err := r.db.Exec("UPDATE projects SET name = ? WHERE id = ?", name, id)
	return err
}

func (r *ProjectRepo) SetCompany(id int64, companyID *int64) error {
	_, err := r.db.Exec("UPDATE projects SET company_id = ? WHERE id = ?", companyID, id)
	return err
}

func (r *ProjectRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM projects WHERE id = ?", id)
	return err
}

type ProjectWithStats struct {
	models.Project
	RepoCount   int
	TaskCount   int
	CommitCount int
}

func (r *ProjectRepo) GetAllWithStats() ([]ProjectWithStats, error) {
	query := `
		SELECT
			p.id, p.name, p.company_id, p.created_at, c.name,
			COUNT(DISTINCT r.id) as repo_count,
			COUNT(DISTINCT t.id) as task_count,
			COUNT(DISTINCT rc.id) as commit_count
		FROM projects p
		LEFT JOIN companies c ON c.id = p.company_id
		LEFT JOIN repos r ON r.project_id = p.id
		LEFT JOIN tasks t ON t.project_id = p.id
		LEFT JOIN raw_commits rc ON rc.repo_id = r.id
		GROUP BY p.id
		ORDER BY c.name, p.name
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []ProjectWithStats
	for rows.Next() {
		var p ProjectWithStats
		var companyID sql.NullInt64
		var companyName sql.NullString

		if err := rows.Scan(
			&p.ID, &p.Name, &companyID, &p.CreatedAt, &companyName,
			&p.RepoCount, &p.TaskCount, &p.CommitCount,
		); err != nil {
			return nil, err
		}

		if companyID.Valid {
			p.CompanyID = &companyID.Int64
		}
		p.CompanyName = companyName.String

		projects = append(projects, p)
	}
	return projects, rows.Err()
}
