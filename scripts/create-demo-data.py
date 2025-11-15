#!/usr/bin/env python3
"""
Demo Data Creator for ERPNext
Creates realistic test data for testing the Frappe MCP Server
"""

import requests
import json
from datetime import datetime, timedelta
from typing import Dict, Any

# Configuration
BASE_URL = "http://localhost:8000"  # Adjust if your ERPNext is on a different port
API_KEY = ""  # Leave empty if using OAuth
API_SECRET = ""

class ERPNextClient:
    def __init__(self, base_url: str, api_key: str = "", api_secret: str = ""):
        self.base_url = base_url.rstrip('/')
        self.api_key = api_key
        self.api_secret = api_secret
        self.session = requests.Session()
        
    def _headers(self):
        headers = {"Content-Type": "application/json"}
        if self.api_key and self.api_secret:
            headers["Authorization"] = f"token {self.api_key}:{self.api_secret}"
        return headers
    
    def create_doc(self, doctype: str, data: Dict[str, Any]) -> Dict:
        """Create a document"""
        url = f"{self.base_url}/api/resource/{doctype}"
        response = self.session.post(url, json=data, headers=self._headers())
        response.raise_for_status()
        return response.json().get('data', {})
    
    def get_doc(self, doctype: str, name: str) -> Dict:
        """Get a document"""
        url = f"{self.base_url}/api/resource/{doctype}/{name}"
        response = self.session.get(url, headers=self._headers())
        response.raise_for_status()
        return response.json().get('data', {})
    
    def doc_exists(self, doctype: str, name: str) -> bool:
        """Check if document exists"""
        try:
            self.get_doc(doctype, name)
            return True
        except:
            return False

def create_fiscal_years(client: ERPNextClient):
    """Create fiscal years for testing"""
    print("Creating Fiscal Years...")
    
    fiscal_years = [
        {
            "year": "2023-2024",
            "year_start_date": "2023-04-01",
            "year_end_date": "2024-03-31",
        },
        {
            "year": "2024-2025", 
            "year_start_date": "2024-04-01",
            "year_end_date": "2025-03-31",
        },
    ]
    
    for fy in fiscal_years:
        try:
            if not client.doc_exists("Fiscal Year", fy["year"]):
                client.create_doc("Fiscal Year", fy)
                print(f"  ✓ Created Fiscal Year: {fy['year']}")
            else:
                print(f"  → Fiscal Year already exists: {fy['year']}")
        except Exception as e:
            print(f"  ✗ Failed to create Fiscal Year {fy['year']}: {e}")

def create_companies(client: ERPNextClient):
    """Create demo companies"""
    print("\nCreating Companies...")
    
    companies = [
        {
            "company_name": "VK Corp",
            "abbr": "VK",
            "default_currency": "INR",
            "country": "India",
        },
        {
            "company_name": "ABC Industries",
            "abbr": "ABC",
            "default_currency": "USD",
            "country": "United States",
        },
    ]
    
    for company in companies:
        try:
            if not client.doc_exists("Company", company["company_name"]):
                client.create_doc("Company", company)
                print(f"  ✓ Created Company: {company['company_name']}")
            else:
                print(f"  → Company already exists: {company['company_name']}")
        except Exception as e:
            print(f"  ✗ Failed to create Company {company['company_name']}: {e}")

def create_customers(client: ERPNextClient):
    """Create demo customers"""
    print("\nCreating Customers...")
    
    customers = [
        {
            "customer_name": "Acme Corporation",
            "customer_type": "Company",
            "customer_group": "Commercial",
            "territory": "All Territories",
        },
        {
            "customer_name": "Tech Solutions Inc",
            "customer_type": "Company",
            "customer_group": "Commercial",
            "territory": "All Territories",
        },
        {
            "customer_name": "Global Traders Ltd",
            "customer_type": "Company",
            "customer_group": "Commercial",
            "territory": "All Territories",
        },
        {
            "customer_name": "Retail Mart",
            "customer_type": "Company",
            "customer_group": "Retail",
            "territory": "All Territories",
        },
        {
            "customer_name": "John Doe",
            "customer_type": "Individual",
            "customer_group": "Individual",
            "territory": "All Territories",
        },
    ]
    
    for customer in customers:
        try:
            if not client.doc_exists("Customer", customer["customer_name"]):
                client.create_doc("Customer", customer)
                print(f"  ✓ Created Customer: {customer['customer_name']}")
            else:
                print(f"  → Customer already exists: {customer['customer_name']}")
        except Exception as e:
            print(f"  ✗ Failed to create Customer {customer['customer_name']}: {e}")

def create_suppliers(client: ERPNextClient):
    """Create demo suppliers"""
    print("\nCreating Suppliers...")
    
    suppliers = [
        {
            "supplier_name": "Hardware Suppliers Co",
            "supplier_group": "Hardware",
            "supplier_type": "Company",
        },
        {
            "supplier_name": "Office Supplies Ltd",
            "supplier_group": "Services",
            "supplier_type": "Company",
        },
        {
            "supplier_name": "Tech Components Inc",
            "supplier_group": "Hardware",
            "supplier_type": "Company",
        },
    ]
    
    for supplier in suppliers:
        try:
            if not client.doc_exists("Supplier", supplier["supplier_name"]):
                client.create_doc("Supplier", supplier)
                print(f"  ✓ Created Supplier: {supplier['supplier_name']}")
            else:
                print(f"  → Supplier already exists: {supplier['supplier_name']}")
        except Exception as e:
            print(f"  ✗ Failed to create Supplier {supplier['supplier_name']}: {e}")

def create_items(client: ERPNextClient):
    """Create demo items"""
    print("\nCreating Items...")
    
    items = [
        {
            "item_code": "LAPTOP-001",
            "item_name": "Dell Latitude Laptop",
            "item_group": "Products",
            "stock_uom": "Nos",
            "is_stock_item": 1,
            "standard_rate": 50000,
        },
        {
            "item_code": "MOUSE-001",
            "item_name": "Wireless Mouse",
            "item_group": "Products",
            "stock_uom": "Nos",
            "is_stock_item": 1,
            "standard_rate": 500,
        },
        {
            "item_code": "KEYBOARD-001",
            "item_name": "Mechanical Keyboard",
            "item_group": "Products",
            "stock_uom": "Nos",
            "is_stock_item": 1,
            "standard_rate": 3000,
        },
        {
            "item_code": "MONITOR-001",
            "item_name": "24\" LED Monitor",
            "item_group": "Products",
            "stock_uom": "Nos",
            "is_stock_item": 1,
            "standard_rate": 12000,
        },
        {
            "item_code": "SERVICE-CONSULT",
            "item_name": "IT Consulting Service",
            "item_group": "Services",
            "stock_uom": "Hour",
            "is_stock_item": 0,
            "standard_rate": 5000,
        },
    ]
    
    for item in items:
        try:
            if not client.doc_exists("Item", item["item_code"]):
                client.create_doc("Item", item)
                print(f"  ✓ Created Item: {item['item_name']}")
            else:
                print(f"  → Item already exists: {item['item_name']}")
        except Exception as e:
            print(f"  ✗ Failed to create Item {item['item_name']}: {e}")

def create_projects(client: ERPNextClient):
    """Create demo projects"""
    print("\nCreating Projects...")
    
    projects = [
        {
            "project_name": "Website Redesign",
            "status": "Open",
            "project_type": "Internal",
            "priority": "High",
            "expected_start_date": "2024-01-01",
            "expected_end_date": "2024-06-30",
        },
        {
            "project_name": "Mobile App Development",
            "status": "Open",
            "project_type": "External",
            "priority": "Medium",
            "expected_start_date": "2024-02-01",
            "expected_end_date": "2024-08-31",
        },
        {
            "project_name": "Infrastructure Upgrade",
            "status": "Completed",
            "project_type": "Internal",
            "priority": "High",
            "expected_start_date": "2023-06-01",
            "expected_end_date": "2023-12-31",
            "percent_complete": 100,
        },
    ]
    
    for project in projects:
        try:
            if not client.doc_exists("Project", project["project_name"]):
                client.create_doc("Project", project)
                print(f"  ✓ Created Project: {project['project_name']}")
            else:
                print(f"  → Project already exists: {project['project_name']}")
        except Exception as e:
            print(f"  ✗ Failed to create Project {project['project_name']}: {e}")

def print_summary():
    """Print summary of demo data"""
    print("\n" + "="*60)
    print("✅ DEMO DATA SETUP COMPLETE!")
    print("="*60)
    print("\n📊 You can now test queries like:")
    print("  • 'Show me all customers'")
    print("  • 'List items in stock'")
    print("  • 'Show me projects'")
    print("  • 'Give me P&L statement for VK Corp from Q1 2024'")
    print("  • 'Show me suppliers'")
    print("\n💡 Tips:")
    print("  • Make sure fiscal years are set up for financial reports")
    print("  • You may need to create Chart of Accounts for companies")
    print("  • Add more data through ERPNext UI as needed")
    print("\n" + "="*60)

def main():
    """Main function to create all demo data"""
    print("="*60)
    print("🚀 CREATING DEMO DATA FOR ERPNEXT")
    print("="*60)
    
    # Initialize client
    client = ERPNextClient(BASE_URL, API_KEY, API_SECRET)
    
    # Create data in order (respecting dependencies)
    try:
        create_fiscal_years(client)
        create_companies(client)
        create_customers(client)
        create_suppliers(client)
        create_items(client)
        create_projects(client)
        print_summary()
        
    except requests.exceptions.ConnectionError:
        print("\n❌ ERROR: Could not connect to ERPNext")
        print(f"   Make sure ERPNext is running at {BASE_URL}")
        print("   Check your ERPNext container status")
        
    except requests.exceptions.HTTPError as e:
        print(f"\n❌ HTTP ERROR: {e}")
        print("   Check your API credentials and permissions")
        
    except Exception as e:
        print(f"\n❌ UNEXPECTED ERROR: {e}")

if __name__ == "__main__":
    main()

