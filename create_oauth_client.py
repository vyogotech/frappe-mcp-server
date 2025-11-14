#!/usr/bin/env python3
"""
Script to create OAuth client for MCP Server integration in ERPNext
Run this inside the ERPNext container with:
docker exec -it CONTAINER_ID bench --site SITENAME console < create_oauth_client.py
"""

import frappe
from frappe import _

def create_mcp_oauth_client():
    """Create OAuth client for MCP backend integration"""
    
    # Check if client already exists
    existing = frappe.db.exists("OAuth Client", {"app_name": "MCP Backend Integration"})
    if existing:
        client = frappe.get_doc("OAuth Client", existing)
        print("=" * 60)
        print("OAuth Client already exists!")
        print("=" * 60)
        print(f"App Name: {client.app_name}")
        print(f"Client ID: {client.client_id}")
        print(f"Client Secret: {client.get_password('client_secret')}")
        print("=" * 60)
        return
    
    # Create new OAuth client
    oauth_client = frappe.get_doc({
        "doctype": "OAuth Client",
        "app_name": "MCP Backend Integration",
        "scopes": [
            {"scope": "openid"},
            {"scope": "all"}
        ],
        "grant_type": "Authorization Code",
        "response_type": "Code"
    })
    
    oauth_client.insert(ignore_permissions=True)
    frappe.db.commit()
    
    print("=" * 60)
    print("OAuth Client created successfully!")
    print("=" * 60)
    print(f"App Name: {oauth_client.app_name}")
    print(f"Client ID: {oauth_client.client_id}")
    print(f"Client Secret: {oauth_client.get_password('client_secret')}")
    print("=" * 60)
    print("\nNext steps:")
    print("1. Copy the Client ID and Secret above")
    print("2. Go to: http://localhost:8000/app/mcp-server-settings")
    print("3. Fill in the OAuth credentials")
    print("4. Update your MCP server config.yaml with the Client ID")
    print("=" * 60)

if __name__ == "__main__":
    create_mcp_oauth_client()




