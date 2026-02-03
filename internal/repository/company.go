package repository

import (
	"database/sql"

	"github.com/emilianohg/anchorman/internal/models"
)

type CompanyRepo struct {
	db *sql.DB
}

func NewCompanyRepo(db *sql.DB) *CompanyRepo {
	return &CompanyRepo{db: db}
}

func (r *CompanyRepo) Create(name string) (*models.Company, error) {
	result, err := r.db.Exec("INSERT INTO companies (name) VALUES (?)", name)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *CompanyRepo) GetByID(id int64) (*models.Company, error) {
	var c models.Company
	err := r.db.QueryRow(
		"SELECT id, name, created_at FROM companies WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Name, &c.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CompanyRepo) GetAll() ([]models.Company, error) {
	rows, err := r.db.Query("SELECT id, name, created_at FROM companies ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []models.Company
	for rows.Next() {
		var c models.Company
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}

func (r *CompanyRepo) Update(id int64, name string) error {
	_, err := r.db.Exec("UPDATE companies SET name = ? WHERE id = ?", name, id)
	return err
}

func (r *CompanyRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM companies WHERE id = ?", id)
	return err
}

type CompanyWithStats struct {
	models.Company
	ProjectCount int
	RepoCount    int
	TaskCount    int
}

func (r *CompanyRepo) GetAllWithStats() ([]CompanyWithStats, error) {
	query := `
		SELECT
			c.id, c.name, c.created_at,
			COUNT(DISTINCT p.id) as project_count,
			COUNT(DISTINCT r.id) as repo_count,
			COUNT(DISTINCT t.id) as task_count
		FROM companies c
		LEFT JOIN projects p ON p.company_id = c.id
		LEFT JOIN repos r ON r.project_id = p.id
		LEFT JOIN tasks t ON t.project_id = p.id
		GROUP BY c.id
		ORDER BY c.name
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []CompanyWithStats
	for rows.Next() {
		var c CompanyWithStats
		if err := rows.Scan(
			&c.ID, &c.Name, &c.CreatedAt,
			&c.ProjectCount, &c.RepoCount, &c.TaskCount,
		); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}
