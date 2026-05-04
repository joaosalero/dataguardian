from __future__ import annotations

import argparse
from getpass import getpass
from pathlib import Path
import sys


ROOT_DIR = Path(__file__).resolve().parents[1]
BACKEND_DIR = ROOT_DIR / "backend"
sys.path.insert(0, str(BACKEND_DIR))

import app.models  # noqa: E402
from app.auth import create_user_record, validate_password_strength  # noqa: E402
from app.core.database import Base, SessionLocal, engine  # noqa: E402
from app.models.user import User  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create a DataGuardian user.")
    parser.add_argument("--username", required=True, help="Username or email.")
    parser.add_argument("--admin", action="store_true", help="Create an admin user.")
    parser.add_argument(
        "--must-change-password",
        action="store_true",
        help="Require password change on next sign-in.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    first_entry = getpass("Password: ")
    second_entry = getpass("Confirm password: ")
    if first_entry != second_entry:
        print("Passwords do not match.", file=sys.stderr)
        return 1

    validate_password_strength(first_entry)
    username = args.username.strip().lower()

    Base.metadata.create_all(bind=engine)
    db = SessionLocal()
    try:
        existing_user = db.query(User).filter(User.email == username).one_or_none()
        if existing_user is not None:
            print("User already exists.", file=sys.stderr)
            return 1

        user = create_user_record(
            db,
            username,
            first_entry,
            is_admin=args.admin,
            must_change_password=args.must_change_password,
        )
        db.commit()
        db.refresh(user)
        print(f"User created: id={user.id} username={user.email}")
        return 0
    finally:
        db.close()


if __name__ == "__main__":
    raise SystemExit(main())
