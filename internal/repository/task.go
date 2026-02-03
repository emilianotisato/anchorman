package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/emilianohg/anchorman/internal/models"
)

type TaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(projectID int64, description string, sourceCommits []int64, taskDate time.Time) (*models.Task, error) {
	commitsJSON, err := json.Marshal(sourceCommits)
	if err != nil {
		return nil, err
	}

	result, err := r.db.Exec(`
		INSERT INTO tasks (project_id, description, source_commits, task_date)
		VALUES (?, ?, ?, ?)
	`, projectID, description, string(commitsJSON), taskDate)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *TaskRepo) GetByID(id int64) (*models.Task, error) {
	var t models.Task
	var commitsJSON string

	err := r.db.QueryRow(`
		SELECT t.id, t.project_id, t.description, t.source_commits, t.task_date, t.created_at, p.name
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE t.id = ?
	`, id).Scan(
		&t.ID, &t.ProjectID, &t.Description, &commitsJSON, &t.TaskDate, &t.CreatedAt, &t.ProjectName,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(commitsJSON), &t.SourceCommits); err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *TaskRepo) GetByProjectAndDateRange(projectID int64, from, to time.Time) ([]models.Task, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.project_id, t.description, t.source_commits, t.task_date, t.created_at, p.name
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE t.project_id = ? AND t.task_date >= ? AND t.task_date <= ?
		ORDER BY t.task_date ASC
	`, projectID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

func (r *TaskRepo) GetByCompanyAndDateRange(companyID int64, from, to time.Time) ([]models.Task, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.project_id, t.description, t.source_commits, t.task_date, t.created_at, p.name
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE p.company_id = ? AND t.task_date >= ? AND t.task_date <= ?
		ORDER BY p.name, t.task_date ASC
	`, companyID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

func (r *TaskRepo) GetByDateRange(from, to time.Time) ([]models.Task, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.project_id, t.description, t.source_commits, t.task_date, t.created_at, p.name
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		WHERE t.task_date >= ? AND t.task_date <= ?
		ORDER BY p.name, t.task_date ASC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

func (r *TaskRepo) scanTasks(rows *sql.Rows) ([]models.Task, error) {
	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var commitsJSON string

		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Description, &commitsJSON, &t.TaskDate, &t.CreatedAt, &t.ProjectName,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(commitsJSON), &t.SourceCommits); err != nil {
			return nil, err
		}

		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

func (r *TaskRepo) DeleteByProjectID(projectID int64) error {
	_, err := r.db.Exec("DELETE FROM tasks WHERE project_id = ?", projectID)
	return err
}
