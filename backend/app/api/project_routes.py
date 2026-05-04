from fastapi import APIRouter, Depends, Response, status
from sqlalchemy.orm import Session

from app.core.database import get_db
from app.models.user import User
from app.schemas.project import ProjectCreate, ProjectResponse
from app.auth import get_current_user
from app.services.project_service import ProjectService


router = APIRouter(prefix="/projects", tags=["projects"])


async def get_project_service(db: Session = Depends(get_db)) -> ProjectService:
    return ProjectService(db)


@router.get(
    "",
    response_model=list[ProjectResponse],
    summary="List projects",
    description="Lists projects owned by the authenticated user.",
    responses={401: {"description": "Missing or invalid bearer token."}},
)
async def list_projects(
    current_user: User = Depends(get_current_user),
    service: ProjectService = Depends(get_project_service),
) -> list[ProjectResponse]:
    return service.list_projects(current_user.id)


@router.post(
    "",
    response_model=ProjectResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Create project",
    description="Creates a project scoped to the authenticated user.",
    responses={
        201: {"description": "Project created."},
        400: {"description": "Project input is invalid."},
        401: {"description": "Missing or invalid bearer token."},
    },
)
async def create_project(
    payload: ProjectCreate,
    current_user: User = Depends(get_current_user),
    service: ProjectService = Depends(get_project_service),
) -> ProjectResponse:
    return service.create_project(payload, current_user.id)


@router.get(
    "/{project_id}",
    response_model=ProjectResponse,
    summary="Get project",
    description="Returns one project only when it belongs to the authenticated user.",
    responses={
        401: {"description": "Missing or invalid bearer token."},
        404: {"description": "Project not found or not accessible."},
    },
)
async def get_project(
    project_id: int,
    current_user: User = Depends(get_current_user),
    service: ProjectService = Depends(get_project_service),
) -> ProjectResponse:
    return service.get_project(project_id, current_user.id)


@router.delete(
    "/{project_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    summary="Delete project",
    description="Deletes a project owned by the authenticated user.",
    responses={
        204: {"description": "Project deleted."},
        401: {"description": "Missing or invalid bearer token."},
        404: {"description": "Project not found or not accessible."},
    },
)
async def delete_project(
    project_id: int,
    current_user: User = Depends(get_current_user),
    service: ProjectService = Depends(get_project_service),
) -> Response:
    service.delete_project(project_id, current_user.id)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
