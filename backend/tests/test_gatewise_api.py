"""
Gatewise API Test Suite - Complete Testing
Tests: Policies CRUD, JIT Access, Drift Detection, License, Connectors, WebSocket
NOTE: All APIs are MOCKED in server.py - data stored in memory
"""

import pytest
import requests
import os
import json
import websocket
import threading
import time

BASE_URL = os.environ.get('REACT_APP_BACKEND_URL', '').rstrip('/')
API = f"{BASE_URL}/api"

# ============ Health & Status Tests ============

class TestHealthAndStatus:
    """Basic health and status endpoint tests"""
    
    def test_health_endpoint(self):
        """Test /api/health returns healthy status"""
        response = requests.get(f"{API}/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "time" in data
        print(f"✓ Health endpoint OK: {data['status']}")
    
    def test_status_endpoint(self):
        """Test /api/status returns full status with version and features"""
        response = requests.get(f"{API}/status")
        assert response.status_code == 200
        data = response.json()
        
        # Validate structure
        assert "version" in data
        assert "license" in data
        assert "features" in data
        assert "connectors" in data
        assert "policyCount" in data
        assert "jitGrantsActive" in data
        assert "driftsDetected" in data
        assert "databaseOk" in data
        
        # Validate values
        assert data["version"] == "0.3.0-alpha"
        assert data["license"] in ["community", "enterprise"]
        assert isinstance(data["features"], list)
        assert len(data["connectors"]) >= 3
        print(f"✓ Status endpoint OK - version: {data['version']}, license: {data['license']}")


# ============ Policies CRUD Tests ============

class TestPoliciesCRUD:
    """Policy management endpoints tests"""
    
    @pytest.fixture(autouse=True)
    def setup_cleanup(self):
        """Cleanup test policies after each test"""
        yield
        # Delete test policies
        response = requests.get(f"{API}/policies")
        if response.status_code == 200:
            policies = response.json().get("policies", [])
            for policy in policies:
                if policy["name"].startswith("TEST_"):
                    requests.delete(f"{API}/policies/{policy['name']}")
    
    def test_get_policies_list(self):
        """Test GET /api/policies returns list"""
        response = requests.get(f"{API}/policies")
        assert response.status_code == 200
        data = response.json()
        assert "policies" in data
        assert "total" in data
        assert isinstance(data["policies"], list)
        print(f"✓ Policies list OK - total: {data['total']}")
    
    def test_create_policy_with_yaml(self):
        """Test POST /api/policies creates a new policy from YAML"""
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_create-policy
  namespace: test-ns
spec:
  team: test-team
  members:
    - test@example.com
  targets:
    - type: kubernetes
      name: k8s-test
      kubernetes:
        namespace: test-ns
        resources: [pods]
        verbs: [get, list]
"""
        response = requests.post(f"{API}/policies", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert "name" in data
        assert data["name"] == "TEST_create-policy"
        print(f"✓ Policy created: {data['name']}")
        
        # Verify persistence with GET
        get_response = requests.get(f"{API}/policies/TEST_create-policy")
        assert get_response.status_code == 200
        policy = get_response.json()
        assert policy["name"] == "TEST_create-policy"
        assert policy["namespace"] == "test-ns"
        assert policy["team"] == "test-team"
        print(f"✓ Policy verified via GET: {policy['name']}")
    
    def test_get_single_policy(self):
        """Test GET /api/policies/{name} returns single policy"""
        # First create a policy
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_get-single-policy
  namespace: test
spec:
  team: test
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        response = requests.get(f"{API}/policies/TEST_get-single-policy")
        assert response.status_code == 200
        data = response.json()
        assert data["name"] == "TEST_get-single-policy"
        print(f"✓ Single policy GET OK: {data['name']}")
    
    def test_get_nonexistent_policy_returns_404(self):
        """Test GET /api/policies/{name} returns 404 for non-existent policy"""
        response = requests.get(f"{API}/policies/nonexistent-policy-12345")
        assert response.status_code == 404
        print("✓ Non-existent policy returns 404")
    
    def test_update_policy(self):
        """Test PUT /api/policies/{name} updates existing policy"""
        # Create policy first
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_update-policy
  namespace: original-ns
spec:
  team: original
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        # Update the policy
        updated_yaml = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_update-policy
  namespace: updated-ns
spec:
  team: updated
  members: [new@example.com]
  targets: []
"""
        response = requests.put(f"{API}/policies/TEST_update-policy", json={"yaml": updated_yaml})
        assert response.status_code == 200
        print("✓ Policy update succeeded")
        
        # Verify update with GET
        get_response = requests.get(f"{API}/policies/TEST_update-policy")
        assert get_response.status_code == 200
        policy = get_response.json()
        assert policy["namespace"] == "updated-ns"
        assert policy["team"] == "updated"
        print(f"✓ Policy update verified: namespace={policy['namespace']}, team={policy['team']}")
    
    def test_delete_policy(self):
        """Test DELETE /api/policies/{name} removes policy"""
        # Create policy first
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_delete-policy
  namespace: test
spec:
  team: test
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        # Delete the policy
        response = requests.delete(f"{API}/policies/TEST_delete-policy")
        assert response.status_code == 200
        print("✓ Policy delete succeeded")
        
        # Verify deletion with GET
        get_response = requests.get(f"{API}/policies/TEST_delete-policy")
        assert get_response.status_code == 404
        print("✓ Policy deletion verified (returns 404)")
    
    def test_reconcile_policy(self):
        """Test POST /api/policies/{name}/reconcile triggers reconciliation"""
        # Create policy first
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_reconcile-policy
  namespace: test
spec:
  team: test
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        response = requests.post(f"{API}/policies/TEST_reconcile-policy/reconcile")
        assert response.status_code == 200
        data = response.json()
        assert "message" in data
        print(f"✓ Policy reconciliation OK: {data['message']}")


# ============ Policy Validation Tests ============

class TestPolicyValidation:
    """Policy validation endpoint tests"""
    
    def test_validate_valid_policy(self):
        """Test /api/policies/validate with valid YAML"""
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: valid-policy
  namespace: test
spec:
  team: platform
  members: [user@example.com]
  targets: []
"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == True
        assert len(data.get("errors", [])) == 0
        print("✓ Valid policy validation passed")
    
    def test_validate_invalid_apiversion(self):
        """Test /api/policies/validate rejects invalid apiVersion"""
        yaml_content = """apiVersion: invalid/v1
kind: AccessPolicy
metadata:
  name: invalid-policy
spec: {}
"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == False
        assert len(data["errors"]) > 0
        print(f"✓ Invalid apiVersion correctly rejected: {data['errors']}")
    
    def test_validate_invalid_kind(self):
        """Test /api/policies/validate rejects invalid kind"""
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: InvalidKind
metadata:
  name: invalid-kind
spec: {}
"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == False
        print(f"✓ Invalid kind correctly rejected")
    
    def test_validate_missing_name(self):
        """Test /api/policies/validate rejects policy without name"""
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata: {}
spec: {}
"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == False
        errors = [e["field"] for e in data.get("errors", [])]
        assert "metadata.name" in errors
        print("✓ Missing name correctly rejected")
    
    def test_validate_forbidden_name(self):
        """Test /api/policies/validate rejects forbidden names"""
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: kube-system
spec: {}
"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": yaml_content})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == False
        print("✓ Forbidden name (kube-system) correctly rejected")
    
    def test_validate_empty_yaml(self):
        """Test /api/policies/validate handles empty YAML"""
        response = requests.post(f"{API}/policies/validate", json={"yaml": ""})
        assert response.status_code == 200
        data = response.json()
        assert data["valid"] == False
        print("✓ Empty YAML correctly rejected")


# ============ JIT Access Tests (Enterprise Feature) ============

class TestJITAccess:
    """JIT (Just-In-Time) Access endpoint tests - requires Enterprise license"""
    
    @pytest.fixture(autouse=True)
    def setup_enterprise_license(self):
        """Set up enterprise license for JIT tests"""
        # First, set enterprise license
        requests.post(f"{API}/license", json={"key": "ENT-TEST-KEY-123"})
        
        # Create a test policy for JIT
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_jit-target-policy
  namespace: production
spec:
  team: platform
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        yield
        
        # Cleanup
        requests.delete(f"{API}/policies/TEST_jit-target-policy")
        requests.post(f"{API}/license", json={"key": "community"})
    
    def test_get_jit_grants_list(self):
        """Test GET /api/jit returns list of grants"""
        response = requests.get(f"{API}/jit")
        assert response.status_code == 200
        data = response.json()
        assert "grants" in data
        assert "total" in data
        print(f"✓ JIT grants list OK - total: {data['total']}")
    
    def test_request_jit_access(self):
        """Test POST /api/jit/request creates a new JIT grant"""
        jit_request = {
            "policyName": "TEST_jit-target-policy",
            "grantee": "test@example.com",
            "reason": "Testing JIT access",
            "duration": "2h"
        }
        response = requests.post(f"{API}/jit/request", json=jit_request)
        assert response.status_code == 200
        data = response.json()
        
        # Validate response structure
        assert "id" in data
        assert data["policyName"] == "TEST_jit-target-policy"
        assert data["grantee"] == "test@example.com"
        assert data["status"] == "active"
        assert "expiresAt" in data
        print(f"✓ JIT grant created: {data['id']}, expires: {data['expiresAt']}")
        
        # Store for cleanup
        return data["id"]
    
    def test_jit_requires_enterprise_license(self):
        """Test JIT request returns 403 without enterprise license"""
        # Reset to community license
        requests.post(f"{API}/license", json={"key": "community"})
        
        jit_request = {
            "policyName": "some-policy",
            "grantee": "test@example.com",
            "duration": "1h"
        }
        response = requests.post(f"{API}/jit/request", json=jit_request)
        assert response.status_code == 403
        print("✓ JIT correctly requires Enterprise license (403)")
        
        # Restore enterprise license for other tests
        requests.post(f"{API}/license", json={"key": "ENT-TEST-KEY-123"})
    
    def test_jit_requires_valid_policy(self):
        """Test JIT request returns 404 for non-existent policy"""
        jit_request = {
            "policyName": "non-existent-policy",
            "grantee": "test@example.com",
            "duration": "1h"
        }
        response = requests.post(f"{API}/jit/request", json=jit_request)
        assert response.status_code == 404
        print("✓ JIT correctly rejects non-existent policy (404)")


# ============ Drift Detection Tests (Enterprise Feature) ============

class TestDriftDetection:
    """Drift detection endpoint tests - requires Enterprise license"""
    
    @pytest.fixture(autouse=True)
    def setup_enterprise_license(self):
        """Set up enterprise license for drift tests"""
        requests.post(f"{API}/license", json={"key": "ENT-TEST-KEY-123"})
        yield
        requests.post(f"{API}/license", json={"key": "community"})
    
    def test_get_drifts_list(self):
        """Test GET /api/drift returns list of drifts"""
        response = requests.get(f"{API}/drift")
        assert response.status_code == 200
        data = response.json()
        assert "drifts" in data
        assert "total" in data
        print(f"✓ Drift list OK - total: {data['total']}")
    
    def test_simulate_drift(self):
        """Test POST /api/drift/simulate creates a drift event"""
        response = requests.post(f"{API}/drift/simulate?policy_name=TEST_drift-policy")
        assert response.status_code == 200
        data = response.json()
        
        assert "id" in data
        assert data["status"] == "detected"
        assert "detectedAt" in data
        print(f"✓ Drift simulated: {data['id']}, status: {data['status']}")
        return data["id"]
    
    def test_repair_drift(self):
        """Test POST /api/drift/{id}/repair repairs a drift"""
        # First simulate a drift
        sim_response = requests.post(f"{API}/drift/simulate?policy_name=TEST_repair-drift")
        drift_id = sim_response.json()["id"]
        
        # Repair the drift
        response = requests.post(f"{API}/drift/{drift_id}/repair")
        assert response.status_code == 200
        data = response.json()
        assert "message" in data
        print(f"✓ Drift repaired: {drift_id}")
        
        # Verify drift status changed
        get_response = requests.get(f"{API}/drift")
        drifts = get_response.json()["drifts"]
        repaired_drift = next((d for d in drifts if d["id"] == drift_id), None)
        assert repaired_drift is not None
        assert repaired_drift["status"] == "repaired"
        print(f"✓ Drift repair verified: status={repaired_drift['status']}")
    
    def test_repair_all_drifts(self):
        """Test POST /api/drift/repair-all repairs all detected drifts"""
        # Simulate multiple drifts
        requests.post(f"{API}/drift/simulate?policy_name=TEST_bulk-repair-1")
        requests.post(f"{API}/drift/simulate?policy_name=TEST_bulk-repair-2")
        
        response = requests.post(f"{API}/drift/repair-all")
        assert response.status_code == 200
        print("✓ Repair all drifts succeeded")
    
    def test_drift_requires_enterprise_for_simulate(self):
        """Test drift simulation requires Enterprise license"""
        requests.post(f"{API}/license", json={"key": "community"})
        
        response = requests.post(f"{API}/drift/simulate?policy_name=test")
        assert response.status_code == 403
        print("✓ Drift simulate correctly requires Enterprise license (403)")
        
        requests.post(f"{API}/license", json={"key": "ENT-TEST-KEY-123"})
    
    def test_repair_nonexistent_drift(self):
        """Test repair returns 404 for non-existent drift"""
        response = requests.post(f"{API}/drift/nonexistent-drift-id/repair")
        assert response.status_code == 404
        print("✓ Non-existent drift repair returns 404")


# ============ Connectors Tests ============

class TestConnectors:
    """Connectors endpoint tests"""
    
    def test_get_connectors_list(self):
        """Test GET /api/connectors returns all connectors"""
        response = requests.get(f"{API}/connectors")
        assert response.status_code == 200
        data = response.json()
        
        assert "connectors" in data
        connectors = data["connectors"]
        assert len(connectors) >= 3
        
        connector_types = [c["type"] for c in connectors]
        assert "kubernetes" in connector_types
        assert "kafka" in connector_types
        assert "kong" in connector_types
        print(f"✓ Connectors list OK: {connector_types}")
    
    def test_kubernetes_connector_available(self):
        """Test Kubernetes connector is always available"""
        response = requests.get(f"{API}/connectors")
        connectors = response.json()["connectors"]
        
        k8s = next(c for c in connectors if c["type"] == "kubernetes")
        assert k8s["status"] == "available"
        assert k8s["enabled"] == True
        print(f"✓ Kubernetes connector: status={k8s['status']}, enabled={k8s['enabled']}")
    
    def test_kafka_kong_locked_in_community(self):
        """Test Kafka and Kong are locked in Community license"""
        # Ensure community license
        requests.post(f"{API}/license", json={"key": "community"})
        
        response = requests.get(f"{API}/connectors")
        connectors = response.json()["connectors"]
        
        kafka = next(c for c in connectors if c["type"] == "kafka")
        kong = next(c for c in connectors if c["type"] == "kong")
        
        assert kafka["status"] == "locked"
        assert kafka["enabled"] == False
        assert kong["status"] == "locked"
        assert kong["enabled"] == False
        print("✓ Kafka/Kong correctly locked in Community mode")
    
    def test_kafka_kong_available_in_enterprise(self):
        """Test Kafka and Kong are available in Enterprise license"""
        requests.post(f"{API}/license", json={"key": "ENT-TEST-KEY"})
        
        response = requests.get(f"{API}/connectors")
        connectors = response.json()["connectors"]
        
        kafka = next(c for c in connectors if c["type"] == "kafka")
        kong = next(c for c in connectors if c["type"] == "kong")
        
        assert kafka["status"] == "available"
        assert kafka["enabled"] == True
        assert kong["status"] == "available"
        assert kong["enabled"] == True
        print("✓ Kafka/Kong correctly available in Enterprise mode")
        
        # Reset to community
        requests.post(f"{API}/license", json={"key": "community"})


# ============ License Tests ============

class TestLicense:
    """License management endpoint tests"""
    
    def test_get_license(self):
        """Test GET /api/license returns current license info"""
        response = requests.get(f"{API}/license")
        assert response.status_code == 200
        data = response.json()
        
        assert "tier" in data
        assert "features" in data
        assert data["tier"] in ["community", "enterprise"]
        print(f"✓ License info OK: tier={data['tier']}, features={len(data['features'])}")
    
    def test_set_enterprise_license(self):
        """Test POST /api/license sets enterprise license with ENT- prefix"""
        response = requests.post(f"{API}/license", json={"key": "ENT-ENTERPRISE-KEY"})
        assert response.status_code == 200
        data = response.json()
        
        assert data["tier"] == "enterprise"
        assert len(data["features"]) > 5  # Enterprise has more features
        print(f"✓ Enterprise license set: features count={len(data['features'])}")
        
        # Verify features include enterprise-only features
        assert "jit_access" in data["features"]
        assert "self_healing" in data["features"]
        print("✓ Enterprise features verified (jit_access, self_healing)")
    
    def test_set_community_license(self):
        """Test POST /api/license resets to community without ENT- prefix"""
        # First set enterprise
        requests.post(f"{API}/license", json={"key": "ENT-TEST"})
        
        # Then reset to community
        response = requests.post(f"{API}/license", json={"key": "invalid-key"})
        assert response.status_code == 200
        data = response.json()
        
        assert data["tier"] == "community"
        print(f"✓ Community license set: tier={data['tier']}")


# ============ Audit Logs Tests ============

class TestAuditLogs:
    """Audit log endpoint tests"""
    
    def test_get_audit_logs(self):
        """Test GET /api/audit returns audit logs"""
        response = requests.get(f"{API}/audit")
        assert response.status_code == 200
        data = response.json()
        
        assert "logs" in data
        assert "total" in data
        print(f"✓ Audit logs OK: total={data['total']}")
    
    def test_audit_log_created_on_policy_create(self):
        """Test audit log is created when policy is created"""
        # Create a policy
        yaml_content = """apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: TEST_audit-test-policy
spec:
  team: test
  members: []
  targets: []
"""
        requests.post(f"{API}/policies", json={"yaml": yaml_content})
        
        # Check audit logs
        response = requests.get(f"{API}/audit")
        logs = response.json()["logs"]
        
        # Find CREATE log for our policy
        create_log = next((l for l in logs if l["action"] == "CREATE" and l["policy"] == "TEST_audit-test-policy"), None)
        assert create_log is not None
        print(f"✓ Audit log created for policy creation: {create_log['action']}")
        
        # Cleanup
        requests.delete(f"{API}/policies/TEST_audit-test-policy")


# ============ WebSocket Tests ============

class TestWebSocket:
    """WebSocket connectivity tests"""
    
    def test_websocket_connection(self):
        """Test WebSocket connection can be established"""
        ws_url = BASE_URL.replace("https://", "wss://").replace("http://", "ws://") + "/ws"
        
        received_messages = []
        connection_established = threading.Event()
        
        def on_message(ws, message):
            received_messages.append(message)
            try:
                data = json.loads(message)
                if data.get("type") == "connected":
                    connection_established.set()
            except:
                pass
        
        def on_open(ws):
            print("WebSocket opened")
        
        def on_error(ws, error):
            print(f"WebSocket error: {error}")
        
        ws = websocket.WebSocketApp(ws_url,
                                    on_message=on_message,
                                    on_open=on_open,
                                    on_error=on_error)
        
        # Run WebSocket in background thread
        ws_thread = threading.Thread(target=ws.run_forever, kwargs={"ping_interval": 10})
        ws_thread.daemon = True
        ws_thread.start()
        
        # Wait for connection (max 5 seconds)
        connected = connection_established.wait(timeout=5)
        
        ws.close()
        
        assert connected, "WebSocket connection not established within timeout"
        assert len(received_messages) > 0, "No messages received from WebSocket"
        
        # Parse initial message
        initial_msg = json.loads(received_messages[0])
        assert initial_msg["type"] == "connected"
        assert "policyCount" in initial_msg.get("data", {})
        print(f"✓ WebSocket connection OK: {initial_msg}")


# ============ CLI Tests ============

class TestCLI:
    """CLI binary tests"""
    
    def test_cli_version(self):
        """Test gatewise-cli --version command"""
        import subprocess
        result = subprocess.run(["/app/gatewise/bin/gatewise-cli", "--version"], 
                               capture_output=True, text=True, timeout=10)
        
        assert result.returncode == 0
        assert "0.3.0-alpha" in result.stdout or "0.3.0" in result.stdout
        print(f"✓ CLI version: {result.stdout.strip()}")


if __name__ == "__main__":
    pytest.main([__file__, "-v", "--tb=short"])
