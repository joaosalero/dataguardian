from fastapi import FastAPI


app = FastAPI(title="DataGuardian Backend")


@app.get("/health")
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
