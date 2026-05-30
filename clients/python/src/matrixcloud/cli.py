"""Matrix Cloud command-line interface (``matrixcloud`` / ``mxc``)."""

from __future__ import annotations

import json
import os
from typing import Optional

import typer
from rich.console import Console
from rich.table import Table

from . import config as cfg
from .client import DEFAULT_BASE_URL, MatrixCloud
from .errors import MatrixCloudError

app = typer.Typer(no_args_is_help=True, add_completion=False, help="Matrix Cloud — control your runtime from the terminal.")
sandbox_app = typer.Typer(no_args_is_help=True, help="Drive MCP sandboxes.")
app.add_typer(sandbox_app, name="sandbox")
console = Console()


def _base(url: Optional[str]) -> str:
    return url or os.environ.get("MATRIXCLOUD_URL") or cfg.load().get("base_url") or DEFAULT_BASE_URL


def _client(url: Optional[str] = None) -> MatrixCloud:
    token = os.environ.get("MATRIXCLOUD_TOKEN") or cfg.load().get("token")
    return MatrixCloud(base_url=_base(url), token=token)


def _fail(msg: str) -> None:
    console.print(f"[bold red]✗[/] {msg}")
    raise typer.Exit(1)


@app.command()
def login(
    email: str = typer.Option(..., prompt=True),
    password: str = typer.Option(..., prompt=True, hide_input=True),
    url: Optional[str] = typer.Option(None, help="Runtime base URL"),
) -> None:
    """Sign in and store a session token."""
    c = _client(url)
    try:
        out = c.login(email, password)
    except MatrixCloudError as e:
        _fail(f"login failed: {e}")
    cfg.save(c.base_url, c.token, email)
    u = out["user"]
    console.print(f"[bold green]✓[/] signed in as [b]{u['email']}[/] · {u['role']} · {u['workspace']}")


@app.command()
def signup(
    name: str = typer.Option(..., prompt=True),
    email: str = typer.Option(..., prompt=True),
    password: str = typer.Option(..., prompt=True, hide_input=True, confirmation_prompt=True),
    workspace: Optional[str] = typer.Option(None, help="Workspace (tenant) name"),
    url: Optional[str] = typer.Option(None),
) -> None:
    """Create a workspace + owner account."""
    c = _client(url)
    try:
        out = c.signup(name, email, password, workspace)
    except MatrixCloudError as e:
        _fail(f"signup failed: {e}")
    cfg.save(c.base_url, c.token, email)
    u = out["user"]
    console.print(f"[bold green]✓[/] workspace [b]{u['workspace']}[/] created — signed in as {u['email']}")


@app.command()
def logout(all_sessions: bool = typer.Option(False, "--all", help="Sign out of every session")) -> None:
    """Sign out and clear stored credentials."""
    try:
        _client().logout(all_sessions)
    except MatrixCloudError:
        pass
    cfg.clear()
    console.print("[green]✓[/] signed out")


@app.command()
def me() -> None:
    """Show the authenticated user."""
    try:
        u = _client().me()
    except MatrixCloudError as e:
        _fail(str(e))
    console.print(f"[b]{u['name']}[/] <{u['email']}> · {u['role']} · workspace [b]{u['workspace']}[/]")


@app.command()
def status(url: Optional[str] = typer.Option(None)) -> None:
    """Runtime health + capabilities (live)."""
    c = _client(url)
    try:
        h = c.health()
        caps = c.capabilities()
    except MatrixCloudError as e:
        _fail(f"runtime unreachable at {c.base_url}: {e}")
    console.print(f"[bold green]●[/] [b]{h['runtime_id']}[/] · mode [b]{h['mode']}[/] · v{h['version']}  ([dim]{c.base_url}[/])")
    console.print("  capabilities: " + ", ".join(caps.get("capabilities", [])))
    limits = caps.get("limits", {})
    console.print(f"  limits: max_ttl={limits.get('max_ttl_seconds')}s · max_jobs={limits.get('max_concurrent_jobs')}")


@app.command()
def jobs(url: Optional[str] = typer.Option(None)) -> None:
    """List jobs (newest first)."""
    try:
        rows = _client(url).list_jobs()
    except MatrixCloudError as e:
        _fail(str(e))
    if not rows:
        console.print("[dim]no jobs yet[/]")
        return
    t = Table("Job", "Type", "Status", "Created", box=None, header_style="bold")
    colour = {"complete": "green", "running": "cyan", "error": "red", "expired": "yellow", "queued": "dim", "cancelled": "yellow"}
    for j in rows:
        st = j.get("status", "")
        t.add_row(j.get("job_id", ""), j.get("type", ""), f"[{colour.get(st, 'white')}]{st}[/]", (j.get("created_at") or "").replace("T", " ").replace("Z", ""))
    console.print(t)


@app.command()
def inspect(model: str, revision: str = typer.Option("main"), url: Optional[str] = typer.Option(None)) -> None:
    """Resolve a model's metadata via model.inspect (live)."""
    try:
        meta = _client(url).inspect_model(model, revision)
    except MatrixCloudError as e:
        _fail(str(e))
    t = Table(show_header=False, box=None)
    for k in ("model", "pipeline_tag", "library_name", "model_type", "license", "estimated_parameters", "recommended_runtime", "requires_gpu"):
        if k in meta:
            t.add_row(f"[dim]{k}[/]", str(meta[k]))
    console.print(t)


@sandbox_app.command("start")
def sandbox_start(
    entity_id: str = typer.Argument(..., help="e.g. mcp_server:filesystem"),
    command: str = typer.Option(..., "--cmd", help="start command, e.g. 'npx -y @modelcontextprotocol/server-filesystem /tmp'"),
    ttl: int = typer.Option(600),
    url: Optional[str] = typer.Option(None),
) -> None:
    """Start a sandbox session and stream its lifecycle until ready."""
    c = _client(url)
    try:
        s = c.create_sandbox(entity_id, command, ttl_seconds=ttl)
    except MatrixCloudError as e:
        _fail(str(e))
    sid = s["session_id"]
    console.print(f"[green]✓[/] sandbox [b]{sid}[/] starting (job {s.get('job_id')})")
    try:
        for ev in c.stream_sandbox_events(sid):
            console.print(f"  [dim]{ev.get('step','')}[/] {ev.get('message','')}")
            if ev.get("step") == "ready":
                break
            if ev.get("status") in ("error", "expired"):
                break
    except MatrixCloudError:
        pass
    console.print(f"tools: mxc sandbox tools {sid}")


@sandbox_app.command("tools")
def sandbox_tools(session_id: str, url: Optional[str] = typer.Option(None)) -> None:
    """List the tools exposed by a sandbox."""
    try:
        tools = _client(url).sandbox_tools(session_id)
    except MatrixCloudError as e:
        _fail(str(e))
    t = Table("Tool", "Description", box=None, header_style="bold")
    for tool in tools:
        t.add_row(tool.get("name", ""), (tool.get("description", "") or "")[:80])
    console.print(t)


@sandbox_app.command("call")
def sandbox_call(
    session_id: str,
    name: str,
    args: str = typer.Option("{}", "--args", help="JSON arguments"),
    url: Optional[str] = typer.Option(None),
) -> None:
    """Call a tool in a sandbox."""
    try:
        arguments = json.loads(args)
    except ValueError:
        _fail("--args must be valid JSON")
    try:
        out = _client(url).call_sandbox_tool(session_id, name, arguments)
    except MatrixCloudError as e:
        _fail(str(e))
    console.print_json(data=out)


@sandbox_app.command("stop")
def sandbox_stop(session_id: str, url: Optional[str] = typer.Option(None)) -> None:
    """Delete a sandbox session."""
    try:
        _client(url).delete_sandbox(session_id)
    except MatrixCloudError as e:
        _fail(str(e))
    console.print(f"[green]✓[/] stopped {session_id}")


if __name__ == "__main__":
    app()
