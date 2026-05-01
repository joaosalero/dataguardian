from fastapi import FastAPI

from app.core.config import settings
from app.core.database import init_db


app = FastAPI(title=settings.app_name)


@app.on_event("startup")
async def on_startup() -> None:
    init_db()


@app.get("/health")
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
