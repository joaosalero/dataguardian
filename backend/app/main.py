from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.audit_routes import router as audit_router
from app.api.project_routes import router as project_router
from app.auth import router as auth_router
from app.core.config import settings
from app.core.database import init_db


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    init_db()
    yield


app = FastAPI(title=settings.app_name, lifespan=lifespan)
app.include_router(auth_router)
app.include_router(project_router)
app.include_router(audit_router)


@app.get("/health")
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
