"""
Gatewise Control Plane - Full Featured API
Supports: Policies, JIT Access, Drift Detection, WebSockets, Prometheus Metrics
"""
import os
import asyncio
import json
from datetime import datetime, timedelta
from typing import List, Optional, Dict, Any
from fastapi import FastAPI, HTTPException, Request, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import PlainTextResponse
from pydantic import BaseModel
import yaml

app = FastAPI(title="Gatewise Control Plane", version="0.3.0-alpha")

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ============ In-Memory Data Stores ============
policies_db: Dict[str, dict] = {}
jit_grants: Dict[str, dict] = {}
drift_events: List[dict] = []
audit_logs: List[dict] = []
license_state = {"tier": "community", "key": ""}

# WebSocket connections
ws_connections: List[WebSocket] = []

# ============ WebSocket Manager ============
class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)

    async def broadcast(self, message: dict):
        for connection in self.active_connections:
            try:
                await connection.send_json(message)
            except:
                pass

manager = ConnectionManager()

# ============ Models ============
class PolicyCreate(BaseModel):
    yaml: Optional[str] = None
    policy: Optional[dict] = None

class JITRequest(BaseModel):
    policyName: str
    grantee: str
    reason: Optional[str] = ""
    duration: Optional[str] = "2h"

class LicenseUpdate(BaseModel):
    key: str

# ============ Helper Functions ============
def add_audit_log(action: str, policy: str, actor: str = "system", details: str = ""):
    log = {
        "id": f"audit-{len(audit_logs)+1}",
        "action": action,
        "policy": policy,
        "actor": actor,
        "details": details,
        "timestamp": datetime.utcnow().isoformat()
    }
    audit_logs.append(log)
    asyncio.create_task(manager.broadcast({"type": "audit", "data": log}))
    return log

def parse_duration(duration_str: str) -> timedelta:
    """Parse duration string like '2h', '30m', '1h30m'"""
    total = timedelta()
    if 'h' in duration_str:
        parts = duration_str.split('h')
        total += timedelta(hours=int(parts[0]))
        duration_str = parts[1] if len(parts) > 1 else ""
    if 'm' in duration_str:
        mins = duration_str.replace('m', '')
        if mins:
            total += timedelta(minutes=int(mins))
    return total if total else timedelta(hours=2)

def get_features():
    base = ["k8s_basic_rbac", "k8s_namespace", "k8s_quota", "policy_parsing", "audit_log"]
    if license_state["tier"] == "enterprise":
        base.extend(["k8s_advanced_rbac", "kafka_connector", "kong_connector", 
                     "keycloak_sso", "cerbos_authz", "jit_access", "self_healing", "multi_tenant"])
    return base

# ============ Health & Status ============
@app.get("/api/")
async def root():
    return {"message": "Gatewise Control Plane API", "version": "0.3.0-alpha"}

@app.get("/api/health")
async def health():
    return {"status": "healthy", "time": datetime.utcnow().isoformat()}

@app.get("/api/status")
async def get_status():
    return {
        "version": "0.3.0-alpha",
        "license": license_state["tier"],
        "features": get_features(),
        "connectors": [
            {"type": "kubernetes", "status": "available", "mode": "embedded", "enabled": True},
            {"type": "kafka", "status": "locked" if license_state["tier"] != "enterprise" else "available", 
             "mode": "enterprise", "enabled": license_state["tier"] == "enterprise"},
            {"type": "kong", "status": "locked" if license_state["tier"] != "enterprise" else "available", 
             "mode": "enterprise", "enabled": license_state["tier"] == "enterprise"}
        ],
        "policyCount": len(policies_db),
        "jitGrantsActive": len([g for g in jit_grants.values() if g["status"] == "active"]),
        "driftsDetected": len([d for d in drift_events if d["status"] == "detected"]),
        "databaseOk": True
    }

# ============ Policies ============
@app.get("/api/policies")
async def list_policies(kind: Optional[str] = None, namespace: Optional[str] = None):
    policies = list(policies_db.values())
    if kind:
        policies = [p for p in policies if p.get("kind") == kind]
    if namespace:
        policies = [p for p in policies if p.get("namespace") == namespace]
    return {"policies": policies, "total": len(policies)}

@app.get("/api/policies/{name}")
async def get_policy(name: str):
    if name not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    return policies_db[name]

@app.post("/api/policies")
async def create_policy(data: PolicyCreate):
    if data.yaml:
        try:
            policy_data = yaml.safe_load(data.yaml)
            name = policy_data.get("metadata", {}).get("name", "")
            if not name:
                raise HTTPException(status_code=400, detail="Policy name required")
            
            policies_db[name] = {
                "name": name,
                "kind": policy_data.get("kind", "AccessPolicy"),
                "namespace": policy_data.get("metadata", {}).get("namespace", ""),
                "team": policy_data.get("spec", {}).get("team", ""),
                "targets": len(policy_data.get("spec", {}).get("targets", [])),
                "members": policy_data.get("spec", {}).get("members", []),
                "status": "Synced",
                "createdAt": datetime.utcnow().isoformat(),
                "updatedAt": datetime.utcnow().isoformat(),
                "raw": policy_data
            }
            
            add_audit_log("CREATE", name, "api")
            await manager.broadcast({"type": "policy_created", "data": policies_db[name]})
            
            return {"id": len(policies_db), "name": name, "message": "policy created successfully"}
        except yaml.YAMLError as e:
            raise HTTPException(status_code=400, detail=f"Invalid YAML: {str(e)}")
    
    raise HTTPException(status_code=400, detail="policy or yaml required")

@app.put("/api/policies/{name}")
async def update_policy(name: str, data: PolicyCreate):
    if name not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    
    if data.yaml:
        policy_data = yaml.safe_load(data.yaml)
        policies_db[name].update({
            "kind": policy_data.get("kind", policies_db[name]["kind"]),
            "namespace": policy_data.get("metadata", {}).get("namespace", ""),
            "team": policy_data.get("spec", {}).get("team", ""),
            "targets": len(policy_data.get("spec", {}).get("targets", [])),
            "updatedAt": datetime.utcnow().isoformat(),
            "raw": policy_data
        })
        add_audit_log("UPDATE", name, "api")
        await manager.broadcast({"type": "policy_updated", "data": policies_db[name]})
    
    return {"message": "policy updated successfully"}

@app.delete("/api/policies/{name}")
async def delete_policy(name: str):
    if name in policies_db:
        del policies_db[name]
        add_audit_log("DELETE", name, "api")
        await manager.broadcast({"type": "policy_deleted", "data": {"name": name}})
    return {"message": "policy deleted successfully"}

@app.post("/api/policies/{name}/reconcile")
async def reconcile_policy(name: str):
    if name not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    
    policies_db[name]["status"] = "Synced"
    policies_db[name]["updatedAt"] = datetime.utcnow().isoformat()
    
    add_audit_log("RECONCILE", name, "api", "Force reconciliation triggered")
    await manager.broadcast({"type": "policy_reconciled", "data": policies_db[name]})
    
    return {"message": "policy reconciled successfully", "status": "applied"}

@app.post("/api/policies/validate")
async def validate_policy(data: PolicyCreate):
    if not data.yaml:
        return {"valid": False, "errors": [{"field": "yaml", "message": "YAML content required"}]}
    
    try:
        policy_data = yaml.safe_load(data.yaml)
        errors = []
        warnings = []
        
        # Validate apiVersion
        api_version = policy_data.get("apiVersion", "")
        valid_versions = ["gatewise.io/v1alpha1", "gatewise.io/v1beta1", "gatewise.io/v1"]
        if api_version not in valid_versions:
            errors.append({"field": "apiVersion", "message": f"must be one of: {valid_versions}"})
        
        # Validate kind
        kind = policy_data.get("kind", "")
        valid_kinds = ["AccessPolicy", "TeamBinding", "ResourceQuota"]
        if kind not in valid_kinds:
            errors.append({"field": "kind", "message": f"must be one of: {valid_kinds}"})
        
        # Validate metadata.name
        name = policy_data.get("metadata", {}).get("name", "")
        if not name:
            errors.append({"field": "metadata.name", "message": "name is required"})
        elif not name.replace("-", "").replace("_", "").isalnum() or name != name.lower():
            errors.append({"field": "metadata.name", "message": "must be lowercase alphanumeric with hyphens"})
        
        # Forbidden names
        forbidden = ["kube-system", "kube-public", "default", "admin", "root"]
        if name.lower() in forbidden:
            errors.append({"field": "metadata.name", "message": f"'{name}' is forbidden"})
        
        return {
            "valid": len(errors) == 0,
            "errors": errors,
            "warnings": warnings,
            "policy": policy_data if len(errors) == 0 else None
        }
    except Exception as e:
        return {"valid": False, "errors": [{"field": "yaml", "message": str(e)}]}

# ============ JIT Access ============
@app.get("/api/jit")
async def list_jit_grants(status: Optional[str] = None, policy: Optional[str] = None):
    grants = list(jit_grants.values())
    if status:
        grants = [g for g in grants if g["status"] == status]
    if policy:
        grants = [g for g in grants if g["policyName"] == policy]
    return {"grants": grants, "total": len(grants)}

@app.post("/api/jit/request")
async def request_jit_access(req: JITRequest):
    if license_state["tier"] != "enterprise":
        raise HTTPException(status_code=403, detail="JIT access requires Enterprise license")
    
    if req.policyName not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    
    duration = parse_duration(req.duration)
    now = datetime.utcnow()
    
    grant = {
        "id": f"jit-{len(jit_grants)+1}-{int(now.timestamp())}",
        "policyName": req.policyName,
        "grantee": req.grantee,
        "reason": req.reason,
        "duration": req.duration,
        "createdAt": now.isoformat(),
        "expiresAt": (now + duration).isoformat(),
        "renewals": 0,
        "maxRenewals": 2,
        "status": "active"
    }
    
    jit_grants[grant["id"]] = grant
    add_audit_log("JIT_GRANT", req.policyName, "api", f"Granted to {req.grantee}")
    await manager.broadcast({"type": "jit_granted", "data": grant})
    
    return grant

@app.post("/api/jit/{grant_id}/renew")
async def renew_jit_grant(grant_id: str):
    if grant_id not in jit_grants:
        raise HTTPException(status_code=404, detail="Grant not found")
    
    grant = jit_grants[grant_id]
    if grant["status"] != "active":
        raise HTTPException(status_code=400, detail="Grant is not active")
    
    if grant["renewals"] >= grant["maxRenewals"]:
        raise HTTPException(status_code=400, detail="Maximum renewals reached")
    
    duration = parse_duration(grant["duration"])
    grant["renewals"] += 1
    grant["expiresAt"] = (datetime.utcnow() + duration).isoformat()
    
    add_audit_log("JIT_RENEW", grant["policyName"], "api", f"Renewal {grant['renewals']}/{grant['maxRenewals']}")
    await manager.broadcast({"type": "jit_renewed", "data": grant})
    
    return grant

@app.post("/api/jit/{grant_id}/revoke")
async def revoke_jit_grant(grant_id: str, reason: Optional[str] = ""):
    if grant_id not in jit_grants:
        raise HTTPException(status_code=404, detail="Grant not found")
    
    grant = jit_grants[grant_id]
    grant["status"] = "revoked"
    grant["revokedAt"] = datetime.utcnow().isoformat()
    
    add_audit_log("JIT_REVOKE", grant["policyName"], "api", reason)
    await manager.broadcast({"type": "jit_revoked", "data": grant})
    
    return {"message": "Grant revoked successfully"}

# ============ Drift Detection ============
@app.get("/api/drift")
async def list_drifts(status: Optional[str] = None):
    drifts = drift_events
    if status:
        drifts = [d for d in drifts if d["status"] == status]
    return {"drifts": drifts, "total": len(drifts)}

@app.post("/api/drift/{drift_id}/repair")
async def repair_drift(drift_id: str):
    for drift in drift_events:
        if drift["id"] == drift_id:
            drift["status"] = "repaired"
            drift["repairedAt"] = datetime.utcnow().isoformat()
            add_audit_log("DRIFT_REPAIR", drift["policyName"], "api")
            await manager.broadcast({"type": "drift_repaired", "data": drift})
            return {"message": "Drift repaired successfully"}
    
    raise HTTPException(status_code=404, detail="Drift not found")

@app.post("/api/drift/repair-all")
async def repair_all_drifts():
    now = datetime.utcnow().isoformat()
    for drift in drift_events:
        if drift["status"] == "detected":
            drift["status"] = "repaired"
            drift["repairedAt"] = now
    
    await manager.broadcast({"type": "all_drifts_repaired", "data": {}})
    return {"message": "All drifts repaired"}

# Simulate drift detection (in production this would be from K8s watcher)
@app.post("/api/drift/simulate")
async def simulate_drift(policy_name: str = "test-policy"):
    if license_state["tier"] != "enterprise":
        raise HTTPException(status_code=403, detail="Drift detection requires Enterprise license")
    
    drift = {
        "id": f"drift-{len(drift_events)+1}",
        "policyName": policy_name,
        "targetType": "kubernetes",
        "resourceType": "Role",
        "resourceName": f"{policy_name}-role",
        "driftType": "modified",
        "expected": {"rules": [{"verbs": ["get", "list"]}]},
        "actual": {"rules": [{"verbs": ["get"]}]},
        "detectedAt": datetime.utcnow().isoformat(),
        "status": "detected"
    }
    
    drift_events.append(drift)
    add_audit_log("DRIFT_DETECTED", policy_name, "system", "Role permissions modified")
    await manager.broadcast({"type": "drift_detected", "data": drift})
    
    return drift

# ============ Connectors ============
@app.get("/api/connectors")
async def list_connectors():
    return {
        "connectors": [
            {"type": "kubernetes", "status": "available", "mode": "embedded", "enabled": True},
            {"type": "kafka", "status": "locked" if license_state["tier"] != "enterprise" else "available", 
             "mode": "", "enabled": license_state["tier"] == "enterprise"},
            {"type": "kong", "status": "locked" if license_state["tier"] != "enterprise" else "available", 
             "mode": "", "enabled": license_state["tier"] == "enterprise"}
        ]
    }

@app.get("/api/connectors/{conn_type}/health")
async def connector_health(conn_type: str):
    if conn_type == "kubernetes":
        return {"status": "healthy"}
    if conn_type in ["kafka", "kong"] and license_state["tier"] == "enterprise":
        return {"status": "healthy"}
    raise HTTPException(status_code=404, detail="connector not configured or locked")

# ============ License ============
@app.get("/api/license")
async def get_license():
    return {"tier": license_state["tier"], "features": get_features()}

@app.post("/api/license")
async def set_license(data: LicenseUpdate):
    if data.key.startswith("ENT-"):
        license_state["tier"] = "enterprise"
        license_state["key"] = data.key
    else:
        license_state["tier"] = "community"
        license_state["key"] = ""
    
    await manager.broadcast({"type": "license_updated", "data": {"tier": license_state["tier"]}})
    
    return {"message": "license updated", "tier": license_state["tier"], "features": get_features()}

# ============ Audit Logs ============
@app.get("/api/audit")
async def list_audit(limit: int = 50, policy: Optional[str] = None, action: Optional[str] = None):
    logs = audit_logs
    if policy:
        logs = [l for l in logs if l["policy"] == policy]
    if action:
        logs = [l for l in logs if l["action"] == action]
    return {"logs": logs[-limit:], "total": len(logs)}

# ============ WebSocket ============
@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    try:
        # Send initial state
        await websocket.send_json({
            "type": "connected",
            "data": {
                "policyCount": len(policies_db),
                "jitActive": len([g for g in jit_grants.values() if g["status"] == "active"]),
                "driftsDetected": len([d for d in drift_events if d["status"] == "detected"])
            }
        })
        
        while True:
            data = await websocket.receive_text()
            # Handle ping/pong
            if data == "ping":
                await websocket.send_text("pong")
    except WebSocketDisconnect:
        manager.disconnect(websocket)


# ============ Prometheus Metrics ============
@app.get("/metrics", response_class=PlainTextResponse)
async def metrics():
    """Prometheus metrics endpoint"""
    # Count active JIT grants
    now = datetime.utcnow()
    active_jit = sum(1 for g in jit_grants.values() if datetime.fromisoformat(g["expiresAt"].replace("Z", "")) > now)
    
    # Count drifts
    active_drifts = sum(1 for d in drift_events if d["status"] != "repaired")
    
    # Determine license tier
    license_tier = license_state["tier"]
    tier_value_community = 1 if license_tier == "community" else 0
    tier_value_enterprise = 1 if license_tier == "enterprise" else 0
    
    # Format in Prometheus exposition format
    metrics_output = f"""# HELP gatewise_policies_total Total number of active policies
# TYPE gatewise_policies_total gauge
gatewise_policies_total {len(policies_db)}

# HELP gatewise_jit_grants_active Number of active JIT grants
# TYPE gatewise_jit_grants_active gauge
gatewise_jit_grants_active {active_jit}

# HELP gatewise_drifts_detected Number of configuration drifts detected
# TYPE gatewise_drifts_detected gauge
gatewise_drifts_detected {active_drifts}

# HELP gatewise_api_requests_total Total number of API requests
# TYPE gatewise_api_requests_total counter
gatewise_api_requests_total 0

# HELP gatewise_license_tier Current license tier (0=community, 1=enterprise)
# TYPE gatewise_license_tier gauge
gatewise_license_tier{{tier="community"}} {tier_value_community}
gatewise_license_tier{{tier="enterprise"}} {tier_value_enterprise}

# HELP gatewise_database_connected Database connection status
# TYPE gatewise_database_connected gauge
gatewise_database_connected 1

# HELP gatewise_info Gatewise version info
# TYPE gatewise_info gauge
gatewise_info{{version="0.3.0-alpha"}} 1
"""
    
    return metrics_output

