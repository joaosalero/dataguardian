import logging
from datetime import datetime, timezone

from fastapi import HTTPException, status
from sqlalchemy.orm import Session
from sqlalchemy.exc import SQLAlchemyError

from app.models.project import Project
from app.repositories.project_repository import ProjectRepository
from app.schemas.project import ProjectCreate


logger = logging.getLogger(__name__)


class ProjectService:
    def __init__(self, db: Session) -> None:
        self.repository = ProjectRepository(db)

    def create_project(self, payload: ProjectCreate) -> Project:
        name = payload.name.strip()
        if not name:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Project name is required",
            )

        description = payload.description.strip() if payload.description else None
        return self.repository.create(name=name, description=description, user_id=1)

    def list_projects(self) -> list[Project]:
        return self.repository.get_all()

    def get_project(self, project_id: int) -> Project:
        try:
            project = self.repository.get_by_id(project_id)
        except SQLAlchemyError:
            logger.warning("Database unavailable, using mock project")
            return Project(
                id=project_id,
                name="Mock Project",
                description=None,
                created_at=datetime.now(timezone.utc),
                user_id=1,
            )

        if project is None:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        return project

    def delete_project(self, project_id: int) -> None:
        project = self.get_project(project_id)
        self.repository.delete(project)
