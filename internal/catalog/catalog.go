// Package catalog is the runtime's curated index of installable components
// (MCP servers, agents, tools, models). It is real, backend-served reference
// data — not fabricated popularity metrics.
package catalog

// Item is a catalog entry.
type Item struct {
	ID           string `json:"id"`
	Initials     string `json:"initials"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	KindClass    string `json:"kind_class"`
	Runtime      string `json:"runtime"`
	Transport    string `json:"transport,omitempty"`
	Secrets      bool   `json:"secrets"`
	Network      string `json:"network"`
	Verified     bool   `json:"verified"`
	Sandbox      bool   `json:"sandbox"`
	StartCommand string `json:"start_command,omitempty"`
	Source       string `json:"source"`
	License      string `json:"license"`
	Version      string `json:"version,omitempty"`
	Desc         string `json:"desc"`
}

// Items is the curated catalog. Sandbox-enabled MCP servers carry a real
// start_command that the runtime can launch over stdio.
var Items = []Item{
	{
		ID: "mcp_server:filesystem", Initials: "FS", Name: "Filesystem MCP Server", Kind: "MCP Server", KindClass: "green",
		Runtime: "Node / stdio", Transport: "stdio", Secrets: false, Network: "Disabled", Verified: true, Sandbox: true,
		StartCommand: "npx -y @modelcontextprotocol/server-filesystem /tmp",
		Source:       "GitHub", License: "MIT",
		Desc: "Read, write, and watch files and directories from agent workflows — sandbox-safe with no secrets.",
	},
	{
		ID: "mcp_server:everything", Initials: "EV", Name: "Everything MCP Server", Kind: "MCP Server", KindClass: "green",
		Runtime: "Node / stdio", Transport: "stdio", Secrets: false, Network: "Disabled", Verified: true, Sandbox: true,
		StartCommand: "npx -y @modelcontextprotocol/server-everything",
		Source:       "GitHub", License: "MIT",
		Desc: "Reference MCP server exercising prompts, tools and resources — useful for verifying a runtime.",
	},
	{
		ID: "mcp_server:git", Initials: "GT", Name: "Git MCP Server", Kind: "MCP Server", KindClass: "green",
		Runtime: "Python / stdio", Transport: "stdio", Secrets: false, Network: "Disabled", Verified: true, Sandbox: true,
		StartCommand: "uvx mcp-server-git --repository /tmp",
		Source:       "GitHub", License: "MIT",
		Desc: "Inspect and operate on Git repositories (status, log, diff, commit) as MCP tools.",
	},
	{
		ID: "mcp_server:github", Initials: "GH", Name: "GitHub MCP Server", Kind: "MCP Server", KindClass: "green",
		Runtime: "Node / stdio", Transport: "stdio", Secrets: true, Network: "Allowlist", Verified: true, Sandbox: true,
		StartCommand: "npx -y @modelcontextprotocol/server-github",
		Source:       "GitHub", License: "MIT",
		Desc: "Repositories, pull requests, issues, and code search exposed as MCP tools (needs a token to call the API).",
	},
	{
		ID: "model:qwen2.5-7b", Initials: "QW", Name: "Qwen2.5-7B-Instruct", Kind: "Model", KindClass: "amber",
		Runtime: "vLLM / GPU", Secrets: false, Network: "Disabled", Verified: true, Sandbox: false,
		Source: "Hugging Face", License: "Apache-2.0",
		Desc: "High-quality 7B instruction model. Resolve metadata with model.inspect; recommended runtime vLLM.",
	},
	{
		ID: "model:deepseek-v3", Initials: "DS", Name: "DeepSeek-V3", Kind: "Model", KindClass: "amber",
		Runtime: "vLLM / SGLang", Secrets: false, Network: "Disabled", Verified: true, Sandbox: false,
		Source: "Hugging Face", License: "Review required",
		Desc: "Large MoE model. Import a profile, then attach & install onto a GPU runtime.",
	},
}
