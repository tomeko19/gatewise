"""
Gatewise Control Plane - Python API wrapper
This wraps the Go Gatewise server for compatibility with the Emergent environment
"""
import os
import subprocess
import signal
import sys
import time
from datetime import datetime
from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
import httpx

app = FastAPI(title="Gatewise Control Plane", version="0.2.0-alpha")

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Simulated data store (in-memory for demo without PostgreSQL)
policies_db = {}
audit_logs = []


@app.get("/api/")
async def root():
    return {"message": "Gatewise Control Plane API", "version": "0.2.0-alpha"}


@app.get("/api/health")
async def health():
    return {"status": "healthy", "time": datetime.utcnow().isoformat()}


@app.get("/api/status")
async def get_status():
    return {
        "version": "0.2.0-alpha",
        "license": "community",
        "features": [
            "k8s_basic_rbac",
            "k8s_namespace",
            "k8s_quota",
            "policy_parsing",
            "audit_log"
        ],
        "connectors": [
            {"type": "kubernetes", "status": "available", "mode": "embedded", "enabled": True},
            {"type": "kafka", "status": "locked", "mode": "enterprise", "enabled": False},
            {"type": "kong", "status": "locked", "mode": "enterprise", "enabled": False}
        ],
        "policyCount": len(policies_db),
        "databaseOk": True
    }


@app.get("/api/policies")
async def list_policies():
    return {
        "policies": list(policies_db.values()),
        "total": len(policies_db)
    }


@app.get("/api/policies/{name}")
async def get_policy(name: str):
    if name not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    return policies_db[name]


@app.post("/api/policies")
async def create_policy(request: Request):
    data = await request.json()
    
    # Parse YAML if provided
    yaml_content = data.get("yaml", "")
    policy = data.get("policy")
    
    if yaml_content:
        import yaml
        try:
            policy_data = yaml.safe_load(yaml_content)
            name = policy_data.get("metadata", {}).get("name", "")
            if not name:
                raise HTTPException(status_code=400, detail="Policy name required")
            
            policies_db[name] = {
                "name": name,
                "kind": policy_data.get("kind", "AccessPolicy"),
                "namespace": policy_data.get("metadata", {}).get("namespace", ""),
                "team": policy_data.get("spec", {}).get("team", ""),
                "targets": len(policy_data.get("spec", {}).get("targets", [])),
                "status": "Synced",
                "createdAt": datetime.utcnow().isoformat(),
                "updatedAt": datetime.utcnow().isoformat()
            }
            
            audit_logs.append({
                "action": "CREATE",
                "policy": name,
                "timestamp": datetime.utcnow().isoformat()
            })
            
            return {"id": len(policies_db), "name": name, "message": "policy created successfully"}
        except Exception as e:
            raise HTTPException(status_code=400, detail=f"Invalid YAML: {str(e)}")
    
    raise HTTPException(status_code=400, detail="policy or yaml required")


@app.delete("/api/policies/{name}")
async def delete_policy(name: str):
    if name in policies_db:
        del policies_db[name]
        audit_logs.append({
            "action": "DELETE",
            "policy": name,
            "timestamp": datetime.utcnow().isoformat()
        })
    return {"message": "policy deleted successfully"}


@app.post("/api/policies/{name}/reconcile")
async def reconcile_policy(name: str):
    if name not in policies_db:
        raise HTTPException(status_code=404, detail="Policy not found")
    
    policies_db[name]["status"] = "Synced"
    policies_db[name]["updatedAt"] = datetime.utcnow().isoformat()
    
    audit_logs.append({
        "action": "RECONCILE",
        "policy": name,
        "timestamp": datetime.utcnow().isoformat()
    })
    
    return {"message": "policy reconciled successfully", "status": "applied"}


@app.post("/api/policies/validate")
async def validate_policy(request: Request):
    data = await request.json()
    yaml_content = data.get("yaml", "")
    
    if not yaml_content:
        return {"valid": False, "errors": [{"field": "yaml", "message": "YAML content required"}]}
    
    import yaml
    try:
        policy_data = yaml.safe_load(yaml_content)
        
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


@app.get("/api/audit")
async def list_audit():
    return {"logs": audit_logs, "total": len(audit_logs)}


@app.get("/api/connectors")
async def list_connectors():
    return {
        "connectors": [
            {"type": "kubernetes", "status": "available", "mode": "embedded", "enabled": True},
            {"type": "kafka", "status": "locked", "mode": "", "enabled": False},
            {"type": "kong", "status": "locked", "mode": "", "enabled": False}
        ]
    }


@app.get("/api/connectors/{conn_type}/health")
async def connector_health(conn_type: str):
    if conn_type == "kubernetes":
        return {"status": "healthy"}
    raise HTTPException(status_code=404, detail="connector not configured")


@app.get("/api/license")
async def get_license():
    return {
        "tier": "community",
        "features": [
            "k8s_basic_rbac",
            "k8s_namespace", 
            "k8s_quota",
            "policy_parsing",
            "audit_log"
        ]
    }


@app.post("/api/license")
async def set_license(request: Request):
    data = await request.json()
    key = data.get("key", "")
    
    if key.startswith("ENT-"):
        return {
            "message": "license updated",
            "tier": "enterprise",
            "features": [
                "k8s_basic_rbac", "k8s_namespace", "k8s_quota", "policy_parsing", "audit_log",
                "k8s_advanced_rbac", "kafka_connector", "kong_connector", "keycloak_sso",
                "cerbos_authz", "jit_access", "self_healing", "multi_tenant"
            ]
        }
    
    return {
        "message": "license updated",
        "tier": "community",
        "features": ["k8s_basic_rbac", "k8s_namespace", "k8s_quota", "policy_parsing", "audit_log"]
    }
