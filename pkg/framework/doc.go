package framework

import (
	"encoding/json"
	"fmt"
)

// RenderDocHTML generates the HTML content for the documentation page
func RenderDocHTML() string {
	docs := GetDocs()
	docsJson, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return "Error generating docs"
	}

	return fmt.Sprintf(docTemplate, string(docsJson))
}

const docTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation</title>
    <style>
        :root {
            --primary-color: #2563eb;
            --bg-color: #f8fafc;
            --card-bg: #ffffff;
            --text-primary: #1e293b;
            --text-secondary: #64748b;
            --border-color: #e2e8f0;
            --method-bg: #dbeafe;
            --method-text: #1e40af;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            line-height: 1.5;
            margin: 0;
            padding: 20px;
        }
        .container {
            max-width: 1000px;
            margin: 0 auto;
        }
        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
            background: var(--card-bg);
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .header h1 { margin: 0; font-size: 24px; }
        
        .info-box {
            background: #eff6ff;
            border: 1px solid #bfdbfe;
            border-radius: 8px;
            padding: 15px 20px;
            margin-bottom: 30px;
            color: #1e3a8a;
        }
        .info-row {
            display: flex;
            margin-bottom: 8px;
        }
        .info-row:last-child { margin-bottom: 0; }
        .info-label { font-weight: 600; width: 120px; }
        .info-value { font-family: monospace; }

        .btn {
            background-color: var(--primary-color);
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 6px;
            cursor: pointer;
            font-weight: 500;
            transition: opacity 0.2s;
        }
        .btn:hover { opacity: 0.9; }

        .endpoint-card {
            background: var(--card-bg);
            border-radius: 8px;
            margin-bottom: 12px;
            box-shadow: 0 1px 2px rgba(0,0,0,0.05);
            border: 1px solid var(--border-color);
            overflow: hidden;
        }
        
        /* Summary Header (Always Visible) */
        .endpoint-summary {
            padding: 15px 20px;
            display: flex;
            align-items: center;
            cursor: pointer;
            background: var(--card-bg);
            transition: background 0.1s;
        }
        .endpoint-summary:hover {
            background: #f8fafc;
        }
        
        .method {
            background: var(--method-bg);
            color: var(--method-text);
            padding: 4px 8px;
            border-radius: 4px;
            font-weight: bold;
            font-size: 13px;
            margin-right: 12px;
            min-width: 50px;
            text-align: center;
        }
        .path {
            font-family: monospace;
            font-size: 15px;
            font-weight: 600;
            margin-right: 15px;
            color: #0f172a;
        }
        .description {
            color: var(--text-secondary);
            font-size: 14px;
            flex: 1;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .toggle-icon {
            color: var(--text-secondary);
            font-size: 12px;
            margin-left: 10px;
        }

        /* Details Section (Collapsible) */
        .endpoint-details {
            display: none;
            padding: 0 20px 20px 20px;
            border-top: 1px solid var(--border-color);
            background: #fcfcfc;
        }
        .endpoint-details.open {
            display: block;
        }
        
        .schema-container {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 20px;
            margin-top: 15px;
        }
        .schema-title {
            font-size: 12px;
            font-weight: 600;
            color: var(--text-secondary);
            margin-bottom: 6px;
            text-transform: uppercase;
        }
        pre {
            background: #f1f5f9;
            padding: 12px;
            border-radius: 6px;
            overflow-x: auto;
            margin: 0;
            font-size: 12px;
            border: 1px solid #e2e8f0;
        }
        
        .toast {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: #10b981;
            color: white;
            padding: 12px 24px;
            border-radius: 6px;
            display: none;
            animation: slideIn 0.3s ease;
            z-index: 100;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        @keyframes slideIn {
            from { transform: translateY(100%%); opacity: 0; }
            to { transform: translateY(0); opacity: 1; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div>
                <h1>API Documentation</h1>
                <p style="margin: 5px 0 0; color: var(--text-secondary);">Interactive API Reference</p>
            </div>
            <button class="btn" onclick="copyForAI()">📋 Copy for AI Context</button>
        </div>

        <div class="info-box">
            <div class="info-row">
                <span class="info-label">HTTP API:</span>
                <span class="info-value" id="http-base">Loading...</span>
            </div>
            <div class="info-row">
                <span class="info-label">WebSocket:</span>
                <span class="info-value" id="ws-base">Loading...</span>
            </div>
        </div>
        
        <div id="content"></div>
    </div>

    <div id="toast" class="toast">Copied to clipboard!</div>

    <script>
        // Configuration
        const WS_PORT = 1002;
        const apiDocs = %s;

        // Dynamic Connection Info
        const hostname = window.location.hostname;
        const httpBase = window.location.origin;
        const wsBase = "ws://" + hostname + ":" + WS_PORT;

        document.getElementById('http-base').textContent = httpBase;
        document.getElementById('ws-base').textContent = wsBase;

        function render() {
            const container = document.getElementById('content');
            
            apiDocs.forEach((endpoint, index) => {
                const card = document.createElement('div');
                card.className = 'endpoint-card';
                
                const reqJson = JSON.stringify(endpoint.request, null, 2);
                const respJson = JSON.stringify(endpoint.response, null, 2);
                const cardId = 'card-' + index;

                card.innerHTML = 
                    '<div class="endpoint-summary" onclick="toggleDetails(\'' + cardId + '\')">' +
                        '<span class="method">' + endpoint.method + '</span>' +
                        '<span class="path">' + endpoint.path + '</span>' +
                        '<span class="description">' + endpoint.description + '</span>' +
                        '<span class="toggle-icon">▼</span>' +
                    '</div>' +
                    
                    '<div id="' + cardId + '" class="endpoint-details">' +
                        '<div class="schema-container">' +
                            '<div class="schema-section">' +
                                '<div class="schema-title">Request Body</div>' +
                                '<pre>' + reqJson + '</pre>' +
                            '</div>' +
                            '<div class="schema-section">' +
                                '<div class="schema-title">Response Body</div>' +
                                '<pre>' + respJson + '</pre>' +
                            '</div>' +
                        '</div>' +
                    '</div>';
                    
                container.appendChild(card);
            });
        }

        function toggleDetails(id) {
            const el = document.getElementById(id);
            el.classList.toggle('open');
            // Rotate arrow logic could go here
        }

        function copyForAI() {
            const aiContext = {
                project_info: {
                    title: "Project API & WebSocket Definition",
                    generated_at: new Date().toISOString(),
                    connection_info: {
                        http_base_url: httpBase,
                        websocket_url: wsBase,
                        notes: "All API endpoints are available via HTTP POST and WebSocket. For WebSocket, use the envelope format specified in the 'protocols' section."
                    }
                },
                protocols: {
                    websocket_envelope: {
                        request: {
                            path: "String (matches API path)",
                            id: "String (optional request ID)",
                            data: "Object (matches API request body)"
                        },
                        response: {
                            id: "String (matches request ID)",
                            path: "String",
                            success: "Boolean",
                            message: "String (error message if any)",
                            data: "Object (matches API response body)"
                        }
                    }
                },
                endpoints: apiDocs
            };
            
            const text = "Here is the full API and WebSocket definition for the project. Please use this context for generating client code or understanding the system capabilities:\n\n" + 
                        JSON.stringify(aiContext, null, 2);

            navigator.clipboard.writeText(text).then(() => {
                showToast();
            }).catch(err => {
                console.error('Failed to copy:', err);
                alert('Failed to copy to clipboard');
            });
        }

        function showToast() {
            const toast = document.getElementById('toast');
            toast.style.display = 'block';
            setTimeout(() => {
                toast.style.display = 'none';
            }, 2000);
        }

        // Initialize
        render();
    </script>
</body>
</html>
`
