#!/usr/bin/env python3
"""
Gatewise API Backend Testing Suite
Tests FastAPI endpoints for the Gatewise Control Plane
"""

import requests
import sys
import json
from datetime import datetime

class GatewiseAPITester:
    def __init__(self, base_url="https://unified-gateway-4.preview.emergentagent.com"):
        self.base_url = base_url
        self.api_url = f"{base_url}/api"
        self.tests_run = 0
        self.tests_passed = 0

    def run_test(self, name, method, endpoint, expected_status, data=None, headers=None):
        """Run a single API test"""
        url = f"{self.api_url}/{endpoint}"
        if headers is None:
            headers = {'Content-Type': 'application/json'}

        self.tests_run += 1
        print(f"\n🔍 Testing {name}...")
        print(f"   {method} {url}")
        
        try:
            if method == 'GET':
                response = requests.get(url, headers=headers, timeout=30)
            elif method == 'POST':
                response = requests.post(url, json=data, headers=headers, timeout=30)
            elif method == 'DELETE':
                response = requests.delete(url, headers=headers, timeout=30)

            success = response.status_code == expected_status
            if success:
                self.tests_passed += 1
                print(f"✅ Passed - Status: {response.status_code}")
                try:
                    response_data = response.json()
                    print(f"📄 Response: {json.dumps(response_data, indent=2)}")
                    return True, response_data
                except:
                    print(f"📄 Response: {response.text}")
                    return True, response.text
            else:
                print(f"❌ Failed - Expected {expected_status}, got {response.status_code}")
                print(f"📄 Response: {response.text}")
                return False, {}

        except Exception as e:
            print(f"❌ Failed - Error: {str(e)}")
            return False, {}

    def test_health_check(self):
        """Test health check endpoint"""
        success, response = self.run_test("Health Check", "GET", "health", 200)
        if success and "status" in response and response["status"] == "healthy":
            print("✅ Health endpoint working correctly")
            return True
        return False

    def test_status_endpoint(self):
        """Test status endpoint"""
        success, response = self.run_test("Status Endpoint", "GET", "status", 200)
        if success:
            required_fields = ["version", "license", "features", "connectors", "policyCount", "databaseOk"]
            missing_fields = [field for field in required_fields if field not in response]
            if not missing_fields:
                print("✅ Status endpoint has all required fields")
                
                # Check connectors
                connectors = response.get("connectors", [])
                connector_types = [c.get("type") for c in connectors]
                expected_types = ["kubernetes", "kafka", "kong"]
                if all(t in connector_types for t in expected_types):
                    print("✅ All expected connectors present")
                    return True
                else:
                    print(f"❌ Missing connectors. Found: {connector_types}")
            else:
                print(f"❌ Missing fields: {missing_fields}")
        return False

    def test_list_policies_empty(self):
        """Test list policies endpoint (should be empty initially)"""
        success, response = self.run_test("List Policies (Empty)", "GET", "policies", 200)
        if success and "policies" in response and "total" in response:
            print("✅ Policies endpoint structure correct")
            return True
        return False

    def test_validate_policy_valid(self):
        """Test policy validation with valid YAML"""
        valid_yaml = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: test-policy
  namespace: production
spec:
  team: platform
  members:
    - user@company.com
  targets:
    - type: kubernetes
      name: k8s-prod"""
      
        success, response = self.run_test(
            "Validate Valid Policy", 
            "POST", 
            "policies/validate", 
            200,
            data={"yaml": valid_yaml}
        )
        
        if success and response.get("valid") is True:
            print("✅ Valid policy validation working")
            return True
        return False

    def test_validate_policy_invalid(self):
        """Test policy validation with invalid YAML"""
        invalid_yaml = """apiVersion: invalid
kind: InvalidKind
metadata:
  name: ""
spec: {}"""
      
        success, response = self.run_test(
            "Validate Invalid Policy", 
            "POST", 
            "policies/validate", 
            200,
            data={"yaml": invalid_yaml}
        )
        
        if success and response.get("valid") is False and "errors" in response:
            print("✅ Invalid policy validation working")
            return True
        return False

    def test_create_policy(self):
        """Test policy creation"""
        policy_yaml = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: test-create-policy
  namespace: production
  labels:
    team: platform
spec:
  team: platform
  members:
    - user@company.com
  targets:
    - type: kubernetes
      name: k8s-prod
      kubernetes:
        namespace: production
        resources:
          - pods
          - services
        verbs:
          - get
          - list"""
      
        success, response = self.run_test(
            "Create Policy", 
            "POST", 
            "policies", 
            200,
            data={"yaml": policy_yaml}
        )
        
        if success and "name" in response and response["name"] == "test-create-policy":
            print("✅ Policy creation working")
            return True, response["name"]
        return False, None

    def test_get_policy(self, policy_name):
        """Test get specific policy"""
        success, response = self.run_test(
            "Get Policy", 
            "GET", 
            f"policies/{policy_name}", 
            200
        )
        
        if success and response.get("name") == policy_name:
            print("✅ Get policy working")
            return True
        return False

    def test_reconcile_policy(self, policy_name):
        """Test policy reconciliation"""
        success, response = self.run_test(
            "Reconcile Policy", 
            "POST", 
            f"policies/{policy_name}/reconcile", 
            200
        )
        
        if success and "message" in response:
            print("✅ Policy reconciliation working")
            return True
        return False

    def test_delete_policy(self, policy_name):
        """Test policy deletion"""
        success, response = self.run_test(
            "Delete Policy", 
            "DELETE", 
            f"policies/{policy_name}", 
            200
        )
        
        if success and "message" in response:
            print("✅ Policy deletion working")
            return True
        return False

    def test_list_connectors(self):
        """Test connectors endpoint"""
        success, response = self.run_test("List Connectors", "GET", "connectors", 200)
        if success and "connectors" in response:
            connectors = response["connectors"]
            if len(connectors) == 3:
                # Check kubernetes is available, kafka/kong are locked
                k8s_conn = next((c for c in connectors if c["type"] == "kubernetes"), None)
                kafka_conn = next((c for c in connectors if c["type"] == "kafka"), None)
                kong_conn = next((c for c in connectors if c["type"] == "kong"), None)
                
                if (k8s_conn and k8s_conn["status"] == "available" and 
                    kafka_conn and kafka_conn["status"] == "locked" and
                    kong_conn and kong_conn["status"] == "locked"):
                    print("✅ Connectors endpoint working correctly")
                    return True
        return False

    def test_license_get(self):
        """Test get license endpoint"""
        success, response = self.run_test("Get License", "GET", "license", 200)
        if success and "tier" in response and "features" in response:
            if response["tier"] == "community":
                print("✅ License endpoint working (Community tier)")
                return True
        return False

    def test_license_update_enterprise(self):
        """Test license update with enterprise key"""
        success, response = self.run_test(
            "Update License (Enterprise)", 
            "POST", 
            "license", 
            200,
            data={"key": "ENT-TEST-KEY"}
        )
        
        if success and response.get("tier") == "enterprise":
            print("✅ Enterprise license update working")
            return True
        return False

    def test_license_update_community(self):
        """Test license update with invalid key (should stay community)"""
        success, response = self.run_test(
            "Update License (Invalid Key)", 
            "POST", 
            "license", 
            200,
            data={"key": "INVALID-KEY"}
        )
        
        if success and response.get("tier") == "community":
            print("✅ Invalid license key handling working")
            return True
        return False

    def run_all_tests(self):
        """Run comprehensive API test suite"""
        print("=" * 60)
        print("🚀 Starting Gatewise API Backend Testing Suite")
        print(f"🌐 Base URL: {self.base_url}")
        print("=" * 60)
        
        # Basic health and status tests
        print(f"\n{'='*20} Basic API Tests {'='*20}")
        self.test_health_check()
        self.test_status_endpoint()
        
        # Policy management tests
        print(f"\n{'='*20} Policy Management Tests {'='*20}")
        self.test_list_policies_empty()
        self.test_validate_policy_valid()
        self.test_validate_policy_invalid()
        
        # Create a policy and test CRUD operations
        created_policy_success, policy_name = self.test_create_policy()
        if created_policy_success and policy_name:
            self.test_get_policy(policy_name)
            self.test_reconcile_policy(policy_name)
            self.test_delete_policy(policy_name)
        
        # Connector tests
        print(f"\n{'='*20} Connector Tests {'='*20}")
        self.test_list_connectors()
        
        # License management tests
        print(f"\n{'='*20} License Management Tests {'='*20}")
        self.test_license_get()
        self.test_license_update_enterprise()
        self.test_license_update_community()
        
        print("\n" + "=" * 60)
        print(f"📊 Test Summary: {self.tests_passed}/{self.tests_run} tests passed")
        
        success_rate = (self.tests_passed / self.tests_run) * 100 if self.tests_run > 0 else 0
        
        if self.tests_passed == self.tests_run:
            print("🎉 All API tests passed!")
            return 0
        else:
            print(f"⚠️  {self.tests_run - self.tests_passed} test(s) failed ({success_rate:.1f}% success rate)")
            return 1

def main():
    tester = GatewiseAPITester()
    return tester.run_all_tests()

if __name__ == "__main__":
    sys.exit(main())