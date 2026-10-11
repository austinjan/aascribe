# aascribe

Early-stage Go CLI for project memory, recall, and indexing.

Remote project Q&A is available through the separate `aascribe-mcp` server: a remote MCP client calls `index_project` to create or update A's project index, then `ask_project` to let a server-side pi agent answer with verified source citations. Index updates are loaded immediately. See the [remote MCP and dynamic-IP Tunnel guide](./docs/remote-mcp.zh-TW.md).

## Docs

- [Quick Start](./quick_start.md)
- [Configuration](./docs/configuration.md)
- [Logging](./docs/logging.md)
- [Usage Guide](./docs/USAGE.md)
- [AI Output Shapes](./docs/reference/aascribe_ai_output_shapes.md)
- [Bootstrap Plan](./docs/plans/bootstrap-plan.md)
