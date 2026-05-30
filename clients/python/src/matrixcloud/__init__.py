"""Matrix Cloud — official Python client and CLI.

Talk to a Matrix Runtime control surface (the /v1 API): authenticate, run jobs,
inspect models, and drive MCP sandboxes.

    from matrixcloud import MatrixCloud

    with MatrixCloud("http://localhost:8080") as mc:
        mc.login("you@acme.io", "secret123")
        print(mc.capabilities()["capabilities"])
        print(mc.inspect_model("hf:Qwen/Qwen2.5-7B-Instruct")["recommended_runtime"])
"""

from .client import MatrixCloud
from .errors import AuthError, MatrixCloudError

__all__ = ["MatrixCloud", "MatrixCloudError", "AuthError", "__version__"]
__version__ = "0.1.0"
