from sqlalchemy.orm import Session

from app.models.project import Project


class ProjectRepository:
    def __init__(self, db: Session) -> None:
        self.db = db

    def create(self, name: str, description: str | None, user_id: int) -> Project:
        project = Project(name=name, description=description, user_id=user_id)
        self.db.add(project)
        self.db.commit()
        self.db.refresh(project)
        return project

    def get_all_for_user(self, user_id: int) -> list[Project]:
        return (
            self.db.query(Project)
            .filter(Project.user_id == user_id)
            .order_by(Project.id)
            .all()
        )

    def get_by_id_for_user(self, project_id: int, user_id: int) -> Project | None:
        return (
            self.db.query(Project)
            .filter(Project.id == project_id, Project.user_id == user_id)
            .one_or_none()
        )

    def delete(self, project: Project) -> None:
        self.db.delete(project)
        self.db.commit()
