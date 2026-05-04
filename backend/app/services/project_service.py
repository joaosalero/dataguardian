import logging

from fastapi import HTTPException, status
from sqlalchemy.orm import Session

from app.models.project import Project
from app.repositories.project_repository import ProjectRepository
from app.schemas.project import ProjectCreate


logger = logging.getLogger(__name__)


class ProjectService:
    def __init__(self, db: Session) -> None:
        self.db = db
        self.repository = ProjectRepository(db)

    def create_project(self, payload: ProjectCreate, user_id: int) -> Project:
        name = payload.name.strip()
        if not name:
            logger.warning("Project creation failed: empty name user_id=%s", user_id)
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Project name is required",
            )

        description = payload.description.strip() if payload.description else None
        project = self.repository.create(
            name=name,
            description=description,
            user_id=user_id,
        )
        logger.info("Project created: project_id=%s user_id=%s", project.id, user_id)
        return project

    def list_projects(self, user_id: int) -> list[Project]:
        return self.repository.get_all_for_user(user_id)

    def get_project(self, project_id: int, user_id: int) -> Project:
        """Return a project only when it belongs to the current user.

        Returning 404 for both missing and unauthorized projects avoids exposing
        whether another user's project ID exists.
        """
        project = self.repository.get_by_id_for_user(project_id, user_id)
        if project is None:
            logger.warning(
                "Project not found or not accessible: project_id=%s user_id=%s",
                project_id,
                user_id,
            )
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        logger.info("Project fetched: project_id=%s user_id=%s", project.id, user_id)
        return project

    def delete_project(self, project_id: int, user_id: int) -> None:
        project = self.get_project(project_id, user_id)
        self.repository.delete(project)
