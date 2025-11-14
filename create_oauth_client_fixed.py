#!/usr/bin/env python3
"""
Create OAuth2 Client for MCP Integration in ERPNext
Run this script inside the ERPNext container with the correct Frappe context
"""

import frappe
from frappe import _

def create_oauth_client():
    """Create OAuth2 client for MCP server integration"""
    
    client_id = "g79ghfpol3"
    client_secret = "2f94b43026"
    
    print("=" * 60)
    print("Creating OAuth2 Client for MCP Integration")
    print("=" * 60)
    
    # Check if client already exists
    if frappe.db.exists("OAuth Client", {"client_id": client_id}):
        print(f"⚠️  OAuth Client with ID '{client_id}' already exists.")
        print("Updating existing client...")
        client = frappe.get_doc("OAuth Client", {"client_id": client_id})
    else:
        print(f"✅ Creating new OAuth Client with ID '{client_id}'")
        client = frappe.new_doc("OAuth Client")
    
    # Set client properties
    client.update({
        "app_name": "MCP Integration",
        "client_id": client_id,
        "client_secret": client_secret,
        "grant_type": "Client Credentials",
        "skip_authorization": 1,  # Skip authorization for backend clients
        "redirect_uris": "",  # Not needed for client credentials
    })
    
    # Clear existing scopes and add required ones
    client.scopes = []
    client.append("scopes", {"scope": "openid"})
    client.append("scopes", {"scope": "all"})
    
    # Save the client
    client.save(ignore_permissions=True)
    frappe.db.commit()
    
    print("=" * 60)
    print("✅ OAuth Client Created Successfully!")
    print("=" * 60)
    print(f"App Name: {client.app_name}")
    print(f"Client ID: {client.client_id}")
    print(f"Client Secret: {client_secret}")
    print(f"Grant Type: {client.grant_type}")
    print(f"Scopes: {', '.join([s.scope for s in client.scopes])}")
    print("=" * 60)
    print("\n🎯 Next Steps:")
    print("1. Go to MCP Server Settings in ERPNext")
    print("2. Update these fields:")
    print(f"   - OAuth Client ID: {client_id}")
    print(f"   - OAuth Client Secret: {client_secret}")
    print("   - MCP Server URL: http://frappe-mcp-server:8080")
    print("   - Frappe Base URL: http://erpnext:8000")
    print("3. Click 'Test Connection'")
    print("=" * 60)

if __name__ == "__main__":
    create_oauth_client()

