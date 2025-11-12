"""
title: ERPNext Business Query
author: ERPNext MCP Integration
author_url: https://github.com/your-repo
funding_url: https://github.com/sponsors/your-repo
version: 1.0.0
license: MIT
"""

import requests
import json
from typing import Dict, Any, Optional
from pydantic import BaseModel, Field


class Tools:
    class Valves(BaseModel):
        ERPNEXT_MCP_API_BASE: str = Field(
            default="http://localhost:8080/api/v1",
            description="Base URL for ERPNext MCP API"
        )
        TIMEOUT: int = Field(
            default=30,
            description="Request timeout in seconds"
        )

    def __init__(self):
        self.valves = self.Valves()

    def erpnext_business_query(self, query: str) -> Dict[str, Any]:
        """
        Send natural language business query to ERPNext MCP API
        
        Args:
            query (str): Business question in natural language
            
        Returns:
            Dict containing response, data quality, and insights
        """
        
        try:
            response = requests.post(
                f"{self.valves.ERPNEXT_MCP_API_BASE}/chat",
                json={"message": query},
                headers={"Content-Type": "application/json"},
                timeout=self.valves.TIMEOUT
            )
            
            if response.status_code == 200:
                data = response.json()
                
                # Format the response for better display
                formatted_response = f"""
🔍 **Query**: {query}

📊 **Business Analysis**:
{data.get('response', 'No response available')}

📈 **Data Quality**: {data.get('data_quality', 'Unknown')} ({data.get('data_size', 0)} characters analyzed)
🔧 **Tools Used**: {', '.join(data.get('tools_called', [])) if data.get('tools_called') else 'None'}
⏰ **Generated**: {data.get('timestamp', 'Unknown time')}
"""
                
                return {
                    "success": True,
                    "formatted_response": formatted_response,
                    "raw_data": data
                }
            else:
                return {
                    "success": False,
                    "error": f"API Error: {response.status_code}",
                    "message": response.text,
                    "formatted_response": f"❌ **Error**: Failed to query ERPNext (Status: {response.status_code})"
                }
                
        except Exception as e:
            return {
                "success": False,
                "error": "Connection Error",
                "message": str(e),
                "formatted_response": f"❌ **Connection Error**: Unable to reach ERPNext MCP API\nError: {str(e)}"
            }

    def erpnext_health_check(self) -> Dict[str, Any]:
        """
        Check ERPNext MCP API health and available tools
        
        Returns:
            Dict containing system status and capabilities
        """
        
        try:
            # Health check
            health_response = requests.get(
                f"{self.valves.ERPNEXT_MCP_API_BASE}/health", 
                timeout=10
            )
            
            # Available tools
            tools_response = requests.get(
                f"{self.valves.ERPNEXT_MCP_API_BASE}/tools", 
                timeout=10
            )
            
            if health_response.status_code == 200 and tools_response.status_code == 200:
                health_data = health_response.json()
                tools_data = tools_response.json()
                
                # Format health status
                status_emoji = "✅" if health_data.get("status") == "healthy" else "❌"
                
                formatted_response = f"""
{status_emoji} **System Health Check**

🔗 **ERPNext Connection**: {health_data.get('erpnext_mcp', 'Unknown')}
🤖 **AI Model**: {health_data.get('ollama_model', 'Unknown')}
🔧 **Available Tools**: {health_data.get('tools_count', 0)}
⏰ **Last Check**: {health_data.get('timestamp', 'Unknown')}

🛠️ **Available ERPNext Tools**:
{chr(10).join(f"  • {tool['name']}: {tool.get('description', 'No description')}" for tool in tools_data.get('tools', [])[:10])}
{f"  ... and {len(tools_data.get('tools', [])) - 10} more tools" if len(tools_data.get('tools', [])) > 10 else ""}
"""
                
                return {
                    "success": True,
                    "health_data": health_data,
                    "tools_data": tools_data,
                    "formatted_response": formatted_response
                }
            else:
                return {
                    "success": False,
                    "error": "Health Check Failed",
                    "health_status": health_response.status_code,
                    "tools_status": tools_response.status_code,
                    "formatted_response": f"❌ **Health Check Failed**\nHealth API: {health_response.status_code}\nTools API: {tools_response.status_code}"
                }
                
        except Exception as e:
            return {
                "success": False,
                "error": "Connection Error",
                "message": str(e),
                "formatted_response": f"❌ **Connection Error**: Unable to reach ERPNext MCP API\nError: {str(e)}"
            }

    def erpnext_execute_tool(self, tool_name: str, arguments: Optional[Dict] = None) -> Dict[str, Any]:
        """
        Execute a specific ERPNext tool directly
        
        Args:
            tool_name (str): Name of the ERPNext tool to execute
            arguments (Dict, optional): Tool arguments
            
        Returns:
            Dict containing tool execution results
        """
        
        if arguments is None:
            arguments = {}
        
        try:
            response = requests.post(
                f"{self.valves.ERPNEXT_MCP_API_BASE}/tools/{tool_name}",
                json={"arguments": arguments},
                headers={"Content-Type": "application/json"},
                timeout=self.valves.TIMEOUT
            )
            
            if response.status_code == 200:
                data = response.json()
                
                # Format tool response
                validation_emoji = "✅" if data.get("is_valid_data") else "⚠️"
                
                formatted_response = f"""
🔧 **Tool Execution**: {tool_name}

{validation_emoji} **Data Status**: {"Valid ERPNext data" if data.get('is_valid_data') else "Limited or invalid data"}
📏 **Data Size**: {data.get('data_size', 0)} characters
⏰ **Executed**: {data.get('timestamp', 'Unknown time')}

📊 **Results**:
```
{data.get('result', 'No results available')[:2000]}{'...' if len(data.get('result', '')) > 2000 else ''}
```
"""
                
                return {
                    "success": True,
                    "tool_data": data,
                    "formatted_response": formatted_response
                }
            else:
                return {
                    "success": False,
                    "error": f"Tool Execution Error: {response.status_code}",
                    "message": response.text,
                    "formatted_response": f"❌ **Tool Execution Failed**: {tool_name}\nStatus: {response.status_code}\nError: {response.text}"
                }
                
        except Exception as e:
            return {
                "success": False,
                "error": "Connection Error",
                "message": str(e),
                "formatted_response": f"❌ **Connection Error**: Unable to execute tool {tool_name}\nError: {str(e)}"
            }
