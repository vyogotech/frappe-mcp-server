#!/usr/bin/env python3
"""
Get OAuth2 Token using Authorization Code Grant
Works with Frappe's OAuth2 implementation (no Client Credentials UI)
"""

import requests
import sys
from urllib.parse import urlencode, parse_qs, urlparse
from getpass import getpass

def get_oauth_token_automated(base_url, client_id, client_secret, username, password):
    """
    Automated OAuth2 flow for backend services.
    Uses Authorization Code grant with programmatic authorization.
    """
    
    print("🔐 Getting OAuth2 Token (Authorization Code Flow)")
    print("=" * 60)
    
    redirect_uri = "http://localhost"
    
    # Step 1: Get authorization code
    print("\n1️⃣ Getting authorization code...")
    
    # Construct authorization URL
    auth_params = {
        "client_id": client_id,
        "redirect_uri": redirect_uri,
        "response_type": "code",
        "scope": "openid profile email all"
    }
    
    auth_url = f"{base_url}/api/method/frappe.integrations.oauth2.authorize"
    
    # Create session for cookies
    session = requests.Session()
    
    # Login first
    print("   Logging in to Frappe...")
    login_response = session.post(
        f"{base_url}/api/method/login",
        data={
            "usr": username,
            "pwd": password
        }
    )
    
    if login_response.status_code != 200:
        print(f"❌ Login failed: {login_response.status_code}")
        print(login_response.text)
        return None
    
    print("   ✅ Login successful")
    
    # Get authorization code
    print("   Requesting authorization code...")
    auth_response = session.get(
        auth_url,
        params=auth_params,
        allow_redirects=False
    )
    
    # Check for redirect
    if auth_response.status_code in [301, 302, 303, 307, 308]:
        redirect_location = auth_response.headers.get('Location', '')
        
        # If there's a confirmation page, we need to approve it
        if 'authorize' in redirect_location and 'code=' not in redirect_location:
            print("   Authorization approval required...")
            
            # Approve the authorization
            approve_response = session.post(
                auth_url,
                data={
                    "client_id": client_id,
                    "redirect_uri": redirect_uri,
                    "response_type": "code",
                    "scope": "openid profile email all",
                    "authorize": "1"
                },
                allow_redirects=False
            )
            
            if approve_response.status_code in [301, 302, 303, 307, 308]:
                redirect_location = approve_response.headers.get('Location', '')
        
        # Extract authorization code from redirect
        parsed_url = urlparse(redirect_location)
        query_params = parse_qs(parsed_url.query)
        
        if 'code' in query_params:
            auth_code = query_params['code'][0]
            print(f"   ✅ Authorization code obtained: {auth_code[:20]}...")
        else:
            print(f"❌ No authorization code in redirect: {redirect_location}")
            return None
    else:
        print(f"❌ Unexpected response: {auth_response.status_code}")
        print(auth_response.text)
        return None
    
    # Step 2: Exchange code for token
    print("\n2️⃣ Exchanging code for access token...")
    
    token_response = requests.post(
        f"{base_url}/api/method/frappe.integrations.oauth2.get_token",
        data={
            "grant_type": "authorization_code",
            "code": auth_code,
            "redirect_uri": redirect_uri,
            "client_id": client_id,
            "client_secret": client_secret
        }
    )
    
    if token_response.status_code == 200:
        token_data = token_response.json()
        access_token = token_data.get("access_token")
        expires_in = token_data.get("expires_in", 3600)
        
        print("   ✅ Access token obtained!")
        print(f"   Token (first 50 chars): {access_token[:50]}...")
        print(f"   Expires in: {expires_in} seconds")
        
        return {
            "access_token": access_token,
            "token_type": token_data.get("token_type", "Bearer"),
            "expires_in": expires_in,
            "scope": token_data.get("scope", "")
        }
    else:
        print(f"❌ Token exchange failed: {token_response.status_code}")
        print(token_response.text)
        return None


def get_token_simple(base_url, api_key, api_secret):
    """
    Fallback: Use API key directly (no OAuth2).
    This is what STDIO mode uses.
    """
    print("\n💡 Alternative: Using API Key Authentication")
    print("=" * 60)
    print(f"API Key: {api_key}")
    print(f"API Secret: {api_secret[:10]}...")
    print("\nFor STDIO mode (Cursor), API keys work fine!")
    print("For HTTP mode, OAuth2 is preferred for user-level permissions.")


def main():
    print("=" * 60)
    print("Frappe OAuth2 Token Generator")
    print("(For Authorization Code Grant)")
    print("=" * 60)
    print()
    
    base_url = input("Frappe URL (default: http://localhost:8000): ").strip()
    if not base_url:
        base_url = "http://localhost:8000"
    
    print("\n📋 OAuth2 Client Details:")
    print("(Create at: {}/app/oauth-client)".format(base_url))
    client_id = input("OAuth Client ID: ").strip()
    client_secret = getpass("OAuth Client Secret: ").strip()
    
    print("\n👤 Frappe User Credentials:")
    print("(Used to authorize the OAuth client)")
    username = input("Username: ").strip()
    password = getpass("Password: ").strip()
    
    if not all([client_id, client_secret, username, password]):
        print("❌ All fields are required")
        sys.exit(1)
    
    # Get token
    token = get_oauth_token_automated(base_url, client_id, client_secret, username, password)
    
    if token:
        print("\n" + "=" * 60)
        print("✅ SUCCESS! OAuth2 Token Obtained")
        print("=" * 60)
        print(f"\nAccess Token: {token['access_token']}")
        print(f"Token Type: {token['token_type']}")
        print(f"Expires In: {token['expires_in']} seconds")
        print(f"Scope: {token['scope']}")
        
        print("\n🧪 Test the token:")
        print(f"""
curl -X POST http://localhost:8080/api/v1/chat \\
  -H "Authorization: Bearer {token['access_token']}" \\
  -H "Content-Type: application/json" \\
  -d '{{"message": "List all projects"}}'
""")
        
        print("\n💾 Save for later use:")
        print(f"export OAUTH_TOKEN='{token['access_token']}'")
    else:
        print("\n❌ Failed to get OAuth2 token")
        print("\n💡 Troubleshooting:")
        print("1. Verify OAuth client exists in Frappe")
        print("2. Check 'Skip Authorization' is enabled")
        print("3. Verify username/password are correct")
        print("4. Check Frappe logs for errors")
        
        print("\n📝 For now, you can use API key authentication:")
        print("   Set FRAPPE_API_KEY and FRAPPE_API_SECRET in your config")


if __name__ == "__main__":
    main()




