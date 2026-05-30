"""Exception types for the Matrix Cloud client."""

from __future__ import annotations

from typing import Any, Optional


class MatrixCloudError(Exception):
    """Base error for all client failures."""

    def __init__(self, message: str, status: Optional[int] = None, data: Any = None) -> None:
        super().__init__(message)
        self.status = status
        self.data = data


class AuthError(MatrixCloudError):
    """Raised on 401/403 — missing or invalid credentials."""
