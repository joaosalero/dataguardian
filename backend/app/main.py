from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.core.config import settings


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    yield


app = FastAPI(title=settings.app_name, lifespan=lifespan)


@app.get("/health")
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
