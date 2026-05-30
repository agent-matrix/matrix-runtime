"""Tiny credential store for the CLI (~/.config/matrixcloud/credentials.json)."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any, Dict, Optional


def _config_dir() -> Path:
    override = os.environ.get("MATRIXCLOUD_CONFIG_DIR")
    if override:
        return Path(override)
    return Path.home() / ".config" / "matrixcloud"


def _cred_path() -> Path:
    return _config_dir() / "credentials.json"


def save(base_url: str, token: Optional[str], email: Optional[str] = None) -> None:
    d = _config_dir()
    d.mkdir(parents=True, exist_ok=True)
    path = _cred_path()
    path.write_text(json.dumps({"base_url": base_url, "token": token, "email": email}, indent=2))
    try:
        path.chmod(0o600)
    except OSError:
        pass


def load() -> Dict[str, Any]:
    try:
        return json.loads(_cred_path().read_text())
    except (OSError, ValueError):
        return {}


def clear() -> None:
    try:
        _cred_path().unlink()
    except OSError:
        pass
