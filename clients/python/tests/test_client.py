"""Unit tests for the Matrix Cloud client using an in-memory httpx transport."""

from __future__ import annotations

import json

import httpx
import pytest

from matrixcloud import AuthError, MatrixCloud, MatrixCloudError


def make_handler():
    """A fake runtime: routes a handful of /v1 endpoints."""
    jobs: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        method = request.method
        body = json.loads(request.content) if request.content else {}

        if path == "/v1/health":
            return httpx.Response(200, json={"status": "ok", "runtime_id": "rt_test", "mode": "local-dev", "version": "0.1.0"})
        if path == "/v1/capabilities":
            return httpx.Response(200, json={"capabilities": ["mcp.test", "model.inspect"], "limits": {"max_ttl_seconds": 600, "max_concurrent_jobs": 2}})
        if path == "/v1/auth/login" and method == "POST":
            if body.get("password") == "secret123":
                return httpx.Response(200, json={"token": "tok_abc", "user": {"email": body["email"], "role": "Owner", "workspace": "Acme", "name": "Neo"}})
            return httpx.Response(401, json={"error": "invalid email or password"})
        if path == "/v1/auth/me":
            if request.headers.get("Authorization") == "Bearer tok_abc":
                return httpx.Response(200, json={"user": {"email": "neo@acme.io", "role": "Owner", "workspace": "Acme", "name": "Neo"}})
            return httpx.Response(401, json={"error": "not authenticated"})
        if path == "/v1/jobs" and method == "POST":
            jid = "job_" + str(len(jobs) + 1)
            jobs[jid] = {"job_id": jid, "type": body["type"], "status": "complete",
                         "result": {"model": body.get("payload", {}).get("model"), "recommended_runtime": "vllm"}}
            return httpx.Response(202, json={"job_id": jid, "status": "queued"})
        if path == "/v1/jobs" and method == "GET":
            return httpx.Response(200, json={"jobs": list(jobs.values())})
        if path.startswith("/v1/jobs/") and method == "GET":
            jid = path.rsplit("/", 1)[-1]
            if jid in jobs:
                return httpx.Response(200, json=jobs[jid])
            return httpx.Response(404, json={"error": "job not found"})

        return httpx.Response(404, json={"error": "not found: " + path})

    return handler


def client() -> MatrixCloud:
    return MatrixCloud(base_url="http://rt.test", transport=httpx.MockTransport(make_handler()))


def test_health_and_capabilities():
    with client() as c:
        assert c.health()["status"] == "ok"
        assert "model.inspect" in c.capabilities()["capabilities"]


def test_login_sets_token_and_me():
    with client() as c:
        out = c.login("neo@acme.io", "secret123")
        assert out["token"] == "tok_abc"
        assert c.token == "tok_abc"
        assert c.me()["workspace"] == "Acme"


def test_login_wrong_password_raises_auth_error():
    with client() as c:
        with pytest.raises(AuthError):
            c.login("neo@acme.io", "nope")


def test_me_without_token_raises():
    with client() as c:
        with pytest.raises(AuthError):
            c.me()


def test_run_job_and_inspect_model():
    with client() as c:
        meta = c.inspect_model("hf:Qwen/Qwen2.5-7B-Instruct")
        assert meta["recommended_runtime"] == "vllm"
        assert len(c.list_jobs()) == 1


def test_unknown_path_raises():
    with client() as c:
        with pytest.raises(MatrixCloudError):
            c.get_job("missing")
