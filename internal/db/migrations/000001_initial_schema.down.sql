DROP INDEX IF EXISTS idx_tasks_task_date;
DROP INDEX IF EXISTS idx_tasks_project_id;
DROP INDEX IF EXISTS idx_raw_commits_committed_at;
DROP INDEX IF EXISTS idx_raw_commits_processed;
DROP INDEX IF EXISTS idx_raw_commits_repo_id;
DROP INDEX IF EXISTS idx_repos_project_id;

DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS raw_commits;
DROP TABLE IF EXISTS repos;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS companies;
