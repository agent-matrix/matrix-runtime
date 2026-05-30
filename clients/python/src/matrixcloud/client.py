"""Synchronous HTTP client for the Matrix Runtime /v1 API."""

from __future__ import annotations

import json
import time
from typing import Any, Dict, Iterator, List, Optional

import httpx

from .errors import AuthError, MatrixCloudError

DEFAULT_BASE_URL = "http://localhost:8080"
_TERMINAL = {"complete", "error", "expired", "cancelled"}


class MatrixCloud:
    """A thin, typed client over the runtime's REST API.

    Pass ``transport`` (an ``httpx.BaseTransport``) to inject a mock in tests.
    """

    def __init__(
        self,
        base_url: str = DEFAULT_BASE_URL,
        token: Optional[str] = None,
        timeout: float = 30.0,
        transport: Optional[httpx.BaseTransport] = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
        self._http = httpx.Client(base_url=self.base_url, timeout=timeout, transport=transport)

    # -- lifecycle -------------------------------------------------------
    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> "MatrixCloud":
        return self

    def __exit__(self, *_exc: object) -> None:
        self.close()

    def _headers(self) -> Dict[str, str]:
        h = {"Accept": "application/json"}
        if self.token:
            h["Authorization"] = "Bearer " + self.token
        return h

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        try:
            r = self._http.request(method, path, headers=self._headers(), json=body)
        except httpx.HTTPError as e:  # network / connection error
            raise MatrixCloudError(f"request to {path} failed: {e}") from e
        data: Any = None
        if r.content:
            try:
                data = r.json()
            except ValueError:
                data = r.text
        if r.status_code >= 400:
            msg = data.get("error") if isinstance(data, dict) else (data or f"HTTP {r.status_code}")
            if r.status_code in (401, 403):
                raise AuthError(str(msg), status=r.status_code, data=data)
            raise MatrixCloudError(str(msg), status=r.status_code, data=data)
        return data

    # -- auth ------------------------------------------------------------
    def signup(self, name: str, email: str, password: str, workspace: Optional[str] = None) -> Dict[str, Any]:
        out = self._request("POST", "/v1/auth/signup", {"name": name, "email": email, "password": password, "workspace": workspace})
        self.token = out.get("token")
        return out

    def login(self, email: str, password: str) -> Dict[str, Any]:
        out = self._request("POST", "/v1/auth/login", {"email": email, "password": password})
        self.token = out.get("token")
        return out

    def me(self) -> Dict[str, Any]:
        return self._request("GET", "/v1/auth/me")["user"]

    def logout(self, all_sessions: bool = False) -> None:
        self._request("POST", "/v1/auth/logout" + ("?all=true" if all_sessions else ""), {})
        self.token = None

    # -- platform --------------------------------------------------------
    def health(self) -> Dict[str, Any]:
        return self._request("GET", "/v1/health")

    def capabilities(self) -> Dict[str, Any]:
        return self._request("GET", "/v1/capabilities")

    # -- jobs ------------------------------------------------------------
    def create_job(self, type: str, payload: Optional[Dict[str, Any]] = None, ttl_seconds: Optional[int] = None) -> Dict[str, Any]:
        body: Dict[str, Any] = {"type": type}
        if payload is not None:
            body["payload"] = payload
        if ttl_seconds is not None:
            body["ttl_seconds"] = ttl_seconds
        return self._request("POST", "/v1/jobs", body)

    def get_job(self, job_id: str) -> Dict[str, Any]:
        return self._request("GET", "/v1/jobs/" + job_id)

    def list_jobs(self) -> List[Dict[str, Any]]:
        return self._request("GET", "/v1/jobs").get("jobs", [])

    def cancel_job(self, job_id: str) -> Dict[str, Any]:
        return self._request("DELETE", "/v1/jobs/" + job_id)

    def run_job(
        self,
        type: str,
        payload: Optional[Dict[str, Any]] = None,
        ttl_seconds: Optional[int] = None,
        timeout: float = 60.0,
        poll: float = 0.5,
    ) -> Any:
        """Create a job and block until it reaches a terminal state."""
        job = self.create_job(type, payload, ttl_seconds)
        job_id = job["job_id"]
        deadline = time.time() + timeout
        while time.time() < deadline:
            snap = self.get_job(job_id)
            if snap["status"] in _TERMINAL:
                if snap["status"] != "complete":
                    raise MatrixCloudError(snap.get("error") or snap["status"], data=snap)
                return snap.get("result")
            time.sleep(poll)
        raise MatrixCloudError(f"job {job_id} timed out after {timeout}s")

    def inspect_model(self, model: str, revision: str = "main") -> Dict[str, Any]:
        return self.run_job("model.inspect", {"model": model, "revision": revision})

    def stream_job_events(self, job_id: str) -> Iterator[Dict[str, Any]]:
        yield from self._sse("/v1/jobs/" + job_id + "/events")

    # -- sandboxes -------------------------------------------------------
    def create_sandbox(
        self,
        entity_id: str,
        start_command: str,
        runtime: str = "node",
        transport: str = "stdio",
        ttl_seconds: int = 600,
    ) -> Dict[str, Any]:
        return self._request("POST", "/v1/sandbox/sessions", {
            "entity_id": entity_id,
            "start_command": start_command,
            "runtime": runtime,
            "transport": transport,
            "ttl_seconds": ttl_seconds,
        })

    def get_sandbox(self, session_id: str) -> Dict[str, Any]:
        return self._request("GET", "/v1/sandbox/sessions/" + session_id)

    def sandbox_tools(self, session_id: str) -> List[Dict[str, Any]]:
        return self._request("GET", "/v1/sandbox/sessions/" + session_id + "/tools").get("tools", [])

    def call_sandbox_tool(self, session_id: str, name: str, arguments: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        return self._request("POST", "/v1/sandbox/sessions/" + session_id + "/tools/call", {"name": name, "arguments": arguments or {}})

    def delete_sandbox(self, session_id: str) -> Dict[str, Any]:
        return self._request("DELETE", "/v1/sandbox/sessions/" + session_id)

    def stream_sandbox_events(self, session_id: str) -> Iterator[Dict[str, Any]]:
        yield from self._sse("/v1/sandbox/sessions/" + session_id + "/events")

    # -- internals -------------------------------------------------------
    def _sse(self, path: str) -> Iterator[Dict[str, Any]]:
        with self._http.stream("GET", path, headers=self._headers()) as r:
            if r.status_code >= 400:
                raise MatrixCloudError(f"stream {path}: HTTP {r.status_code}", status=r.status_code)
            for line in r.iter_lines():
                if line.startswith("data: "):
                    try:
                        yield json.loads(line[6:])
                    except ValueError:
                        continue
