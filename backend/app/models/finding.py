from __future__ import annotations

from sqlalchemy import ForeignKey, Integer, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class Finding(Base):
    __tablename__ = "findings"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, index=True)
    audit_run_id: Mapped[int] = mapped_column(
        ForeignKey("audit_runs.id"),
        nullable=False,
    )
    title: Mapped[str] = mapped_column(String, nullable=False)
    description: Mapped[str] = mapped_column(String, nullable=False)
    severity: Mapped[str] = mapped_column(String, nullable=False)
    recommendation: Mapped[str] = mapped_column(String, nullable=False)

    audit_run: Mapped["AuditRun"] = relationship(back_populates="findings")
