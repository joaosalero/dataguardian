from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
import logging

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.api.audit_routes import router as audit_router
from app.api.project_routes import router as project_router
from app.auth import router as auth_router
from app.core.config import settings
from app.core.database import init_db


logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    logger.info("Application startup started: app=%s", settings.app_name)
    init_db()
    logger.info("Application startup completed")
    yield


# CORS is intentionally scoped to local development frontend origins.
# Credentials require explicit origins; avoid using "*" with bearer-token flows.
app = FastAPI(
    title=settings.app_name,
    description=(
        "DataGuardian API for authenticated project management and "
        "database security audit workflows."
    ),
    version="0.1.0",
    lifespan=lifespan,
)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:3000",
        "http://127.0.0.1:3000",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)
app.include_router(auth_router)
app.include_router(project_router)
app.include_router(audit_router)


@app.middleware("http")
async def log_requests(request: Request, call_next):
    """Log method/path/status only; headers, query strings, and bodies may be sensitive."""
    logger.info("Request started: %s %s", request.method, request.url.path)
    response = await call_next(request)
    logger.info(
        "Request finished: %s %s %s",
        request.method,
        request.url.path,
        response.status_code,
    )
    return response


@app.exception_handler(Exception)
async def handle_unexpected_error(request: Request, exc: Exception) -> JSONResponse:
    logger.error(
        "Unhandled exception during %s %s exception_type=%s",
        request.method,
        request.url.path,
        exc.__class__.__name__,
    )
    return JSONResponse(
        status_code=500,
        content={"detail": "Internal server error"},
    )


@app.get(
    "/health",
    tags=["health"],
    summary="Check API health",
    description="Returns a minimal readiness response without exposing configuration.",
    response_description="API health status.",
)
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
