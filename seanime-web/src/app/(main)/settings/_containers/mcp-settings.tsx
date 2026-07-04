import { getServerBaseUrl } from "@/api/client/server-url"
import { SettingsCard, SettingsPageHeader } from "@/app/(main)/settings/_components/settings-card"
import { Alert } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import React from "react"
import { LuCheck, LuCopy } from "react-icons/lu"
import { TbRobot } from "react-icons/tb"
import { toast } from "sonner"

/**
 * Small reusable code block with a copy-to-clipboard button.
 */
function CodeBlock({ code, language, className }: { code: string, language?: string, className?: string }) {
    const [copied, setCopied] = React.useState(false)

    const handleCopy = React.useCallback(async () => {
        try {
            await navigator.clipboard.writeText(code)
            setCopied(true)
            toast.success("Copied to clipboard")
            setTimeout(() => setCopied(false), 1500)
        }
        catch {
            toast.error("Failed to copy")
        }
    }, [code])

    return (
        <div className={cn("relative group/code rounded-md border bg-gray-950/80", className)}>
            {language && (
                <span className="absolute top-2 left-3 text-xs text-(--muted) uppercase tracking-wide select-none">
                    {language}
                </span>
            )}
            <Button
                size="sm"
                intent="gray-subtle"
                className="absolute top-2 right-2 h-7 px-2"
                leftIcon={copied ? <LuCheck className="text-green-400" /> : <LuCopy />}
                onClick={handleCopy}
            >
                {copied ? "Copied" : "Copy"}
            </Button>
            <pre className={cn("overflow-x-auto p-4 text-sm leading-relaxed", language && "pt-8")}>
                <code className="font-mono text-(--foreground) whitespace-pre">{code}</code>
            </pre>
        </div>
    )
}

export function McpSettings() {
    // The MCP server is mounted on the same host/port as the Seanime server.
    const mcpUrl = `${getServerBaseUrl()}/api/v1/mcp`

    const configToml = `[experimental]
mcp = true`

    const claudeCodeSnippet = `claude mcp add --transport http seanime ${mcpUrl}`

    const httpClientSnippet = `{
  "mcpServers": {
    "seanime": {
      "url": "${mcpUrl}"
    }
  }
}`

    const stdioClientSnippet = `{
  "mcpServers": {
    "seanime": {
      "command": "npx",
      "args": ["mcp-remote", "${mcpUrl}"]
    }
  }
}`

    const authSnippet = `{
  "mcpServers": {
    "seanime": {
      "command": "npx",
      "args": [
        "mcp-remote",
        "${mcpUrl}",
        "--header",
        "X-Seanime-Token:\${SEANIME_TOKEN}"
      ]
    }
  }
}`

    return (
        <div className="space-y-4">
            <SettingsPageHeader
                title="MCP Agent"
                description="Let AI agents read your Seanime data through the Model Context Protocol"
                icon={TbRobot}
            />

            <Alert
                intent="warning-basic"
                title="Experimental & read-only"
                description={
                    <div className="space-y-1">
                        <p>
                            The MCP server is an experimental feature. It is <strong>read-only</strong>: agents can search and read
                            AniList data and your collection, but cannot modify anything.
                        </p>
                        <p>It is disabled by default and must be enabled in your <code className="code">config.toml</code>.</p>
                    </div>
                }
            />

            <SettingsCard title="1. Enable the server" description="Edit your config.toml, then restart Seanime">
                <p className="text-sm text-(--muted)">
                    Add the following to your <code className="code">config.toml</code> (located in your data directory), then restart the server.
                </p>
                <CodeBlock code={configToml} language="config.toml" />
            </SettingsCard>

            <SettingsCard title="2. Endpoint" description="Streamable HTTP transport">
                <p className="text-sm text-(--muted)">
                    Once enabled, the MCP server is available at the URL below. It uses the Streamable HTTP transport and runs on the
                    same host and port as Seanime.
                </p>
                <CodeBlock code={mcpUrl} />
            </SettingsCard>

            <SettingsCard title="3. Connect a client" description="Copy the snippet for your agent">
                <div className="space-y-1">
                    <h5 className="font-medium">Claude Code (CLI)</h5>
                    <p className="text-sm text-(--muted)">Register the server in one command:</p>
                    <CodeBlock code={claudeCodeSnippet} language="bash" />
                </div>

                <div className="space-y-1">
                    <h5 className="font-medium">Clients with native HTTP support (Cursor, etc.)</h5>
                    <p className="text-sm text-(--muted)">Add this to the client's MCP config file:</p>
                    <CodeBlock code={httpClientSnippet} language="json" />
                </div>

                <div className="space-y-1">
                    <h5 className="font-medium">Clients that only support stdio (e.g. Claude Desktop)</h5>
                    <p className="text-sm text-(--muted)">
                        Bridge to the HTTP endpoint with <code className="code">mcp-remote</code> (requires Node.js). For Claude Desktop, this goes in{" "}
                        <code className="code">claude_desktop_config.json</code>.
                    </p>
                    <CodeBlock code={stdioClientSnippet} language="json" />
                </div>
            </SettingsCard>

            <SettingsCard title="4. Authentication" description="Only required if a server password is set">
                <p className="text-sm text-(--muted)">
                    If your server has no password (the default for local use), no authentication is needed. If you've set a server
                    password, requests must include the <code className="code">X-Seanime-Token</code> header with your token — for example via{" "}
                    <code className="code">mcp-remote</code>:
                </p>
                <CodeBlock code={authSnippet} language="json" />
                <p className="text-xs text-(--muted)">
                    Set <code className="code">SEANIME_TOKEN</code> in your client's environment to your server token. Some agents let you set request
                    headers directly instead.
                </p>
            </SettingsCard>

            <SettingsCard title="Available tools" description="What the agent can do">
                <ul className="text-sm space-y-1.5">
                    <li><code className="code">search_anime</code> — Search AniList for anime by title.</li>
                    <li><code className="code">search_manga</code> — Search AniList for manga by title.</li>
                    <li><code className="code">get_anime</code> — Get base information for an anime by its media id.</li>
                    <li><code className="code">get_anime_details</code> — Get extended details (characters, relations, recommendations).</li>
                    <li><code className="code">get_anime_collection</code> — Get your anime collection (lists with progress, status, score).</li>
                    <li><code className="code">get_viewer_stats</code> — Get your AniList viewer statistics.</li>
                    <li><code className="code">get_library_files</code> — Get your local library contents (mapped and unmapped files).</li>
                </ul>
            </SettingsCard>
        </div>
    )
}
