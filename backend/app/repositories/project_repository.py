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

    def get_all(self) -> list[Project]:
        return self.db.query(Project).order_by(Project.id).all()

    def get_by_id(self, project_id: int) -> Project | None:
        return self.db.query(Project).filter(Project.id == project_id).first()

    def delete(self, project: Project) -> None:
        self.db.delete(project)
        self.db.commit()
