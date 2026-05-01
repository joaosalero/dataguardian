import logging

from fastapi import HTTPException, status
from sqlalchemy.orm import Session

from app.models.project import Project
from app.models.user import User
from app.repositories.project_repository import ProjectRepository
from app.schemas.project import ProjectCreate


logger = logging.getLogger(__name__)


class ProjectService:
    def __init__(self, db: Session) -> None:
        self.db = db
        self.repository = ProjectRepository(db)

    def create_project(self, payload: ProjectCreate) -> Project:
        name = payload.name.strip()
        if not name:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Project name is required",
            )

        description = payload.description.strip() if payload.description else None
        self._ensure_default_user()
        project = self.repository.create(name=name, description=description, user_id=1)
        logger.info("Project created with ID %s", project.id)
        return project

    def list_projects(self) -> list[Project]:
        return self.repository.get_all()

    def get_project(self, project_id: int) -> Project:
        project = self.repository.get_by_id(project_id)
        if project is None:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        logger.info("Project fetched: %s", project.id)
        return project

    def delete_project(self, project_id: int) -> None:
        project = self.get_project(project_id)
        self.repository.delete(project)

    def _ensure_default_user(self) -> None:
        default_user = self.db.get(User, 1)
        if default_user is not None:
            return

        self.db.add(
            User(
                id=1,
                email="owner@dataguardian.dev",
                password_hash="hashed",
            )
        )
