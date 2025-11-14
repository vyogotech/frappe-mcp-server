#!/usr/bin/env python3
"""
Create OAuth2 Client in Frappe for MCP Server
This script automates the creation of an OAuth2 client for testing.
"""

import requests
import sys
import json
from getpass import getpass

def create_oauth_client(base_url, api_key, api_secret):
    """Create an OAuth2 client in Frappe."""
    
    print("Creating OAuth2 Client in Frappe...")
    print(f"Frappe URL: {base_url}")
    
    # OAuth2 client data
    client_data = {
        "doctype": "OAuth Client",
        "app_name": "MCP Backend Integration",
        "scopes": "openid profile email all",
        "grant_type": "Client Credentials",
        "skip_authorization": 1,
    }
    
    try:
        # Create the OAuth client
        response = requests.post(
            f"{base_url}/api/resource/OAuth Client",
            headers={
                "Authorization": f"token {api_key}:{api_secret}",
                "Content-Type": "application/json",
            },
            json=client_data,
        )
        
        if response.status_code == 200:
            oauth_client = response.json().get("data", {})
            client_id = oauth_client.get("client_id")
            client_secret = oauth_client.get("client_secret")
            
            print("\n✓ OAuth2 Client Created Successfully!")
            print("=" * 50)
            print(f"Client ID:     {client_id}")
            print(f"Client Secret: {client_secret}")
            print("=" * 50)
            print("\nAdd this to your config.yaml:")
            print(f"""
auth:
  enabled: true
  require_auth: false  # Set to true for production
  oauth2:
    token_info_url: "{base_url}/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "{base_url}"
    trusted_clients:
      - "{client_id}"
    validate_remote: true
    timeout: "30s"
""")
            print("\nOr export as environment variables:")
            print(f"""
export OAUTH_CLIENT_ID='{client_id}'
export OAUTH_CLIENT_SECRET='{client_secret}'
export AUTH_ENABLED=true
export AUTH_REQUIRE_AUTH=false
export OAUTH_TOKEN_INFO_URL='{base_url}/api/method/frappe.integrations.oauth2.openid.userinfo'
export OAUTH_ISSUER_URL='{base_url}'
""")
            
            return client_id, client_secret
            
        else:
            print(f"\n✗ Failed to create OAuth client: {response.status_code}")
            print(f"Response: {response.text}")
            return None, None
            
    except Exception as e:
        print(f"\n✗ Error: {e}")
        return None, None


def test_oauth_client(base_url, client_id, client_secret):
    """Test the OAuth2 client by getting a token."""
    
    print("\nTesting OAuth2 Client...")
    
    try:
        response = requests.post(
            f"{base_url}/api/method/frappe.integrations.oauth2.get_token",
            data={
                "grant_type": "client_credentials",
                "client_id": client_id,
                "client_secret": client_secret,
            },
        )
        
        if response.status_code == 200:
            token_data = response.json()
            access_token = token_data.get("access_token")
            
            if access_token:
                print("✓ Successfully obtained access token!")
                print(f"Token (first 50 chars): {access_token[:50]}...")
                print(f"Expires in: {token_data.get('expires_in')} seconds")
                
                # Validate the token
                print("\nValidating token...")
                user_info_response = requests.get(
                    f"{base_url}/api/method/frappe.integrations.oauth2.openid.userinfo",
                    headers={"Authorization": f"Bearer {access_token}"},
                )
                
                if user_info_response.status_code == 200:
                    user_info = user_info_response.json()
                    print("✓ Token is valid!")
                    print(f"User: {user_info.get('sub', 'N/A')}")
                    print(f"Email: {user_info.get('email', 'N/A')}")
                    return True
                else:
                    print(f"✗ Token validation failed: {user_info_response.status_code}")
                    return False
            else:
                print("✗ No access token in response")
                return False
        else:
            print(f"✗ Failed to get token: {response.status_code}")
            print(f"Response: {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Error testing OAuth client: {e}")
        return False


def main():
    """Main function."""
    
    print("=" * 50)
    print("Frappe OAuth2 Client Creator")
    print("=" * 50)
    print()
    
    # Get Frappe connection details
    base_url = input(f"Frappe URL (default: http://localhost:8000): ").strip()
    if not base_url:
        base_url = "http://localhost:8000"
    
    print("\nYou need API credentials to create an OAuth client.")
    print("Get these from Frappe: User Menu → API Access → Generate Keys")
    print()
    
    api_key = input("Frappe API Key: ").strip()
    api_secret = getpass("Frappe API Secret: ").strip()
    
    if not api_key or not api_secret:
        print("✗ API key and secret are required")
        sys.exit(1)
    
    # Create OAuth client
    client_id, client_secret = create_oauth_client(base_url, api_key, api_secret)
    
    if client_id and client_secret:
        # Test the client
        test_oauth_client(base_url, client_id, client_secret)
        
        print("\n✓ Setup complete! You can now test OAuth2 authentication.")
        print("\nRun the test script:")
        print(f"  OAUTH_CLIENT_ID='{client_id}' \\")
        print(f"  OAUTH_CLIENT_SECRET='{client_secret}' \\")
        print("  ./test-oauth.sh")
    else:
        print("\n✗ Failed to create OAuth client")
        sys.exit(1)


if __name__ == "__main__":
    main()

