"""
title: ERPNext Business Query (OAuth2)
author: ERPNext MCP Integration with OAuth2
author_url: https://github.com/your-repo
funding_url: https://github.com/sponsors/your-repo
version: 2.0.0
license: MIT
description: Integration with OAuth2 authentication - No API keys needed!
"""

import requests
import json
from typing import Dict, Any, Optional
from datetime import datetime, timedelta
from pydantic import BaseModel, Field


class Tools:
    class Valves(BaseModel):
        MCP_API_BASE: str = Field(
            default="http://frappe-mcp-server:8080/api/v1",
            description="Base URL for MCP API"
        )
        FRAPPE_BASE_URL: str = Field(
            default="http://localhost:8000",
            description="Frappe instance base URL"
        )
        OAUTH_CLIENT_ID: str = Field(
            default="",
            description="OAuth2 Client ID (from Frappe)"
        )
        OAUTH_CLIENT_SECRET: str = Field(
            default="",
            description="OAuth2 Client Secret (from Frappe)"
        )
        TIMEOUT: int = Field(
            default=30,
            description="Request timeout in seconds"
        )
        CACHE_TOKEN: bool = Field(
            default=True,
            description="Cache OAuth2 tokens to reduce API calls"
        )

    def __init__(self):
        self.valves = self.Valves()
        self._cached_token = None
        self._token_expires_at = None

    def _get_oauth_token(self) -> Optional[str]:
        """
        Get OAuth2 access token using client credentials grant.
        Demonstrates: Web client authentication WITHOUT API keys!
        """
        # Check cache first
        if (self.valves.CACHE_TOKEN and 
            self._cached_token and 
            self._token_expires_at and 
            datetime.now() < self._token_expires_at):
            return self._cached_token
        
        if not self.valves.OAUTH_CLIENT_ID or not self.valves.OAUTH_CLIENT_SECRET:
            return None
        
        try:
            response = requests.post(
                f"{self.valves.FRAPPE_BASE_URL}/api/method/frappe.integrations.oauth2.get_token",
                data={
                    "grant_type": "client_credentials",
                    "client_id": self.valves.OAUTH_CLIENT_ID,
                    "client_secret": self.valves.OAUTH_CLIENT_SECRET,
                },
                headers={"Content-Type": "application/x-www-form-urlencoded"},
                timeout=10
            )
            
            if response.status_code == 200:
                token_data = response.json()
                self._cached_token = token_data.get("access_token")
                # Cache token for expires_in - 60 seconds (buffer)
                expires_in = token_data.get("expires_in", 3600)
                self._token_expires_at = datetime.now() + timedelta(seconds=expires_in - 60)
                return self._cached_token
            else:
                print(f"OAuth2 token request failed: {response.status_code}")
                return None
                
        except Exception as e:
            print(f"Error getting OAuth2 token: {e}")
            return None

    def _make_authenticated_request(self, method: str, endpoint: str, 
                                    json_data: Optional[Dict] = None,
                                    user_context: Optional[Dict] = None) -> requests.Response:
        """
        Make authenticated request to MCP API using OAuth2 token.
        
        This demonstrates how web clients authenticate:
        - NO API keys needed!
        - Use OAuth2 Bearer token
        - Optional user context for trusted clients
        """
        token = self._get_oauth_token()
        
        headers = {"Content-Type": "application/json"}
        
        if token:
            headers["Authorization"] = f"Bearer {token}"
            
            # Add user context if provided (for trusted clients)
            if user_context:
                if "user_id" in user_context:
                    headers["X-MCP-User-ID"] = user_context["user_id"]
                if "user_email" in user_context:
                    headers["X-MCP-User-Email"] = user_context["user_email"]
                if "user_name" in user_context:
                    headers["X-MCP-User-Name"] = user_context["user_name"]
        
        url = f"{self.valves.MCP_API_BASE}{endpoint}"
        
        if method.upper() == "GET":
            return requests.get(url, headers=headers, timeout=self.valves.TIMEOUT)
        elif method.upper() == "POST":
            return requests.post(url, json=json_data, headers=headers, timeout=self.valves.TIMEOUT)
        else:
            raise ValueError(f"Unsupported HTTP method: {method}")

    def erpnext_business_query(self, query: str, user_email: Optional[str] = None) -> str:
        """
        Send natural language business query to ERPNext MCP API.
        Uses OAuth2 authentication - NO API KEYS NEEDED!
        
        Args:
            query (str): Business question in natural language
            user_email (str, optional): User email for context
            
        Returns:
            Formatted response string
        """
        
        try:
            # Prepare user context if email provided
            user_context = None
            if user_email:
                user_context = {
                    "user_email": user_email,
                    "user_id": user_email
                }
            
            # Make authenticated request (with OAuth2 token, not API keys!)
            response = self._make_authenticated_request(
                method="POST",
                endpoint="/chat",
                json_data={"message": query},
                user_context=user_context
            )
            
            if response.status_code == 200:
                data = response.json()
                
                # Check if OAuth2 was used
                auth_method = "🔐 OAuth2" if self._cached_token else "🔑 API Key (fallback)"
                
                # Format the response
                formatted_response = f"""
🔍 **Query**: {query}

{auth_method} **Authentication**: ✅ Secure
{"👤 **User Context**: " + user_email if user_email else "🤖 **Client Context**: Service Account"}

📊 **Business Analysis**:
{data.get('response', 'No response available')}

📈 **Data Quality**: {data.get('data_quality', 'Unknown')} ({data.get('data_size', 0)} characters analyzed)
🔧 **Tools Used**: {', '.join(data.get('tools_called', [])) if data.get('tools_called') else 'None'}
⏰ **Generated**: {data.get('timestamp', 'Unknown time')}
"""
                return formatted_response
                
            elif response.status_code == 401:
                return """
❌ **Authentication Failed**

This demonstrates OAuth2 security in action! 

**Issue**: No valid OAuth2 token available.

**To fix**:
1. Create OAuth2 client in Frappe: http://localhost:8000/app/oauth-client
2. Update this function's Valves with:
   - OAUTH_CLIENT_ID
   - OAUTH_CLIENT_SECRET

**Note**: Unlike STDIO mode (Cursor), web clients like Open WebUI use OAuth2 tokens, not API keys! 🔒
"""
            else:
                return f"""
❌ **Error**: Failed to query ERPNext (Status: {response.status_code})

Response: {response.text}

**Troubleshooting**:
- Check if MCP server is running
- Verify OAuth2 credentials
- Check MCP server logs
"""
                
        except Exception as e:
            return f"""
❌ **Connection Error**: Unable to reach ERPNext MCP API

Error: {str(e)}

**Troubleshooting**:
- Verify MCP_API_BASE: {self.valves.MCP_API_BASE}
- Check network connectivity
- Ensure MCP server is running
"""

    def erpnext_health_check(self) -> str:
        """
        Check ERPNext MCP API health and authentication status.
        
        Returns:
            Formatted health status string
        """
        
        try:
            # Test OAuth2 authentication first
            oauth_status = "❌ Not configured"
            token = None
            
            if self.valves.OAUTH_CLIENT_ID and self.valves.OAUTH_CLIENT_SECRET:
                token = self._get_oauth_token()
                if token:
                    oauth_status = f"✅ Active (Token: {token[:20]}...)"
                else:
                    oauth_status = "❌ Failed to get token"
            
            # Health check
            health_response = self._make_authenticated_request("GET", "/health")
            
            # Available tools
            tools_response = self._make_authenticated_request("GET", "/tools")
            
            if health_response.status_code == 200 and tools_response.status_code == 200:
                health_data = health_response.json()
                tools_data = tools_response.json()
                
                # Format health status
                status_emoji = "✅" if health_data.get("status") == "healthy" else "❌"
                
                formatted_response = f"""
{status_emoji} **System Health Check**

🔐 **OAuth2 Status**: {oauth_status}
🔗 **ERPNext Connection**: {health_data.get('erpnext_mcp', 'Unknown')}
🤖 **AI Model**: {health_data.get('ollama_model', 'Unknown')}
🔧 **Available Tools**: {health_data.get('tools_count', 0)}
⏰ **Last Check**: {health_data.get('timestamp', 'Unknown')}

🛠️ **Available ERPNext Tools** (first 10):
{chr(10).join(f"  • {tool['name']}: {tool.get('description', 'No description')}" for tool in tools_data.get('tools', [])[:10])}
{f"  ... and {len(tools_data.get('tools', [])) - 10} more tools" if len(tools_data.get('tools', [])) > 10 else ""}

💡 **Note**: This integration uses OAuth2 authentication (no API keys needed for web clients!)
"""
                return formatted_response
            else:
                return f"""
❌ **Health Check Failed**
Health API: {health_response.status_code}
Tools API: {tools_response.status_code}

OAuth2 Status: {oauth_status}
"""
                
        except Exception as e:
            return f"""
❌ **Connection Error**: Unable to reach ERPNext MCP API

Error: {str(e)}

**Configuration**:
- MCP Base URL: {self.valves.MCP_API_BASE}
- Frappe URL: {self.valves.FRAPPE_BASE_URL}
- OAuth2 Client: {"Configured" if self.valves.OAUTH_CLIENT_ID else "Not configured"}
"""

    def erpnext_get_projects(self, limit: int = 20) -> str:
        """
        Get list of projects from ERPNext using OAuth2 authentication.
        
        Args:
            limit (int): Maximum number of projects to return
            
        Returns:
            Formatted projects list
        """
        
        try:
            response = self._make_authenticated_request(
                method="POST",
                endpoint="/tools/list_documents",
                json_data={
                    "arguments": {
                        "doctype": "Project",
                        "limit": limit
                    }
                }
            )
            
            if response.status_code == 200:
                data = response.json()
                result = data.get("result", {})
                projects = result.get("data", [])
                
                if projects:
                    projects_list = "\n".join([f"  • {p.get('name', 'N/A')}: {p.get('project_name', 'Unnamed')}" for p in projects])
                    
                    return f"""
📋 **Projects List** (OAuth2 Authenticated)

Total Projects: {result.get('total_count', len(projects))}

{projects_list}

🔐 **Authentication**: OAuth2 Bearer Token (No API keys!)
"""
                else:
                    return """
📋 **Projects List**

No projects found.

🔐 **Authentication**: OAuth2 Bearer Token ✅
"""
            else:
                return f"""
❌ **Failed to fetch projects**: {response.status_code}

{response.text}
"""
                
        except Exception as e:
            return f"""
❌ **Error fetching projects**: {str(e)}
"""

    def clear_token_cache(self) -> str:
        """
        Clear the cached OAuth2 token. Useful for testing or troubleshooting.
        
        Returns:
            Status message
        """
        self._cached_token = None
        self._token_expires_at = None
        return "✅ OAuth2 token cache cleared. Next request will fetch a new token."

