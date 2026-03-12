import { useState, useEffect, useCallback, useRef } from "react";
import "@/App.css";
import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import axios from "axios";

const BACKEND_URL = process.env.REACT_APP_BACKEND_URL;
const API = `${BACKEND_URL}/api`;
const WS_URL = BACKEND_URL.replace("https://", "wss://").replace("http://", "ws://") + "/ws";

// ============ API Client ============
const apiClient = {
  getStatus: () => axios.get(`${API}/status`),
  getHealth: () => axios.get(`${API}/health`),
  getPolicies: () => axios.get(`${API}/policies`),
  getPolicy: (name) => axios.get(`${API}/policies/${name}`),
  createPolicy: (data) => axios.post(`${API}/policies`, data),
  deletePolicy: (name) => axios.delete(`${API}/policies/${name}`),
  reconcilePolicy: (name) => axios.post(`${API}/policies/${name}/reconcile`),
  validatePolicy: (yaml) => axios.post(`${API}/policies/validate`, { yaml }),
  getConnectors: () => axios.get(`${API}/connectors`),
  getLicense: () => axios.get(`${API}/license`),
  setLicense: (key) => axios.post(`${API}/license`, { key }),
  // JIT
  getJITGrants: () => axios.get(`${API}/jit`),
  requestJIT: (data) => axios.post(`${API}/jit/request`, data),
  renewJIT: (id) => axios.post(`${API}/jit/${id}/renew`),
  revokeJIT: (id) => axios.post(`${API}/jit/${id}/revoke`),
  // Drift
  getDrifts: () => axios.get(`${API}/drift`),
  repairDrift: (id) => axios.post(`${API}/drift/${id}/repair`),
  repairAllDrifts: () => axios.post(`${API}/drift/repair-all`),
  simulateDrift: (policy) => axios.post(`${API}/drift/simulate?policy_name=${policy}`),
  // Audit
  getAuditLogs: () => axios.get(`${API}/audit`),
};

// ============ WebSocket Hook ============
const useWebSocket = (onMessage) => {
  const wsRef = useRef(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const connect = () => {
      try {
        wsRef.current = new WebSocket(WS_URL);
        
        wsRef.current.onopen = () => {
          setConnected(true);
          console.log("WebSocket connected");
        };
        
        wsRef.current.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            onMessage(data);
          } catch (e) {
            console.error("Failed to parse WS message", e);
          }
        };
        
        wsRef.current.onclose = () => {
          setConnected(false);
          // Reconnect after 5 seconds
          setTimeout(connect, 5000);
        };
        
        wsRef.current.onerror = () => {
          setConnected(false);
        };
      } catch (e) {
        console.error("WebSocket error", e);
      }
    };

    connect();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [onMessage]);

  return { connected };
};

// ============ Components ============

const Sidebar = ({ wsConnected }) => {
  const location = useLocation();
  
  const links = [
    { path: "/", label: "Dashboard", icon: "grid" },
    { path: "/policies", label: "Policies", icon: "file-text" },
    { path: "/connectors", label: "Connectors", icon: "plug" },
    { path: "/jit", label: "JIT Access", icon: "clock" },
    { path: "/drift", label: "Drift", icon: "alert" },
    { path: "/audit", label: "Audit Logs", icon: "history" },
    { path: "/settings", label: "Settings", icon: "settings" },
  ];

  return (
    <aside className="sidebar" data-testid="sidebar">
      <div className="sidebar-header">
        <div className="logo">
          <span className="logo-icon">⬡</span>
          <span className="logo-text">Gatewise</span>
        </div>
        <span className="version">v0.3.0</span>
      </div>
      <nav className="sidebar-nav">
        {links.map((link) => (
          <Link
            key={link.path}
            to={link.path}
            className={`nav-link ${location.pathname === link.path ? "active" : ""}`}
            data-testid={`nav-${link.label.toLowerCase().replace(" ", "-")}`}
          >
            <span className="nav-icon">{getIcon(link.icon)}</span>
            <span className="nav-label">{link.label}</span>
          </Link>
        ))}
      </nav>
      <div className="sidebar-footer">
        <div className={`ws-status ${wsConnected ? "connected" : ""}`}>
          <span className="ws-dot"></span>
          <span>{wsConnected ? "Live" : "Offline"}</span>
        </div>
      </div>
    </aside>
  );
};

const getIcon = (name) => {
  const icons = {
    grid: "⊞",
    "file-text": "📄",
    plug: "🔌",
    history: "📋",
    settings: "⚙",
    check: "✓",
    x: "✗",
    alert: "⚠",
    clock: "⏱",
    kubernetes: "☸",
    kafka: "◈",
    kong: "◆",
  };
  return icons[name] || "•";
};

// ============ Dashboard Page ============

const Dashboard = ({ realtimeData }) => {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const response = await apiClient.getStatus();
        setStatus(response.data);
      } catch (err) {
        setError("Unable to connect to Gatewise server");
      } finally {
        setLoading(false);
      }
    };
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  if (loading) return <LoadingSpinner />;
  if (error) return <ErrorMessage message={error} />;

  return (
    <div className="dashboard" data-testid="dashboard">
      <header className="page-header">
        <h1>Dashboard</h1>
        <p className="subtitle">Unified Access Layer for K8s, Kafka & Kong</p>
      </header>

      <div className="stats-grid">
        <StatCard
          title="Policies"
          value={status?.policyCount || 0}
          icon="file-text"
          color="blue"
        />
        <StatCard
          title="JIT Active"
          value={realtimeData?.jitActive || status?.jitGrantsActive || 0}
          icon="clock"
          color="green"
        />
        <StatCard
          title="Drifts"
          value={realtimeData?.driftsDetected || status?.driftsDetected || 0}
          icon="alert"
          color={status?.driftsDetected > 0 ? "red" : "green"}
        />
        <StatCard
          title="License"
          value={status?.license || "community"}
          icon="check"
          color="purple"
        />
      </div>

      <div className="dashboard-grid">
        <div className="card connectors-card">
          <h3>Connectors Status</h3>
          <div className="connector-list">
            {status?.connectors?.map((conn) => (
              <ConnectorStatusItem key={conn.type} connector={conn} />
            ))}
          </div>
        </div>

        <div className="card features-card">
          <h3>Enabled Features</h3>
          <div className="feature-list">
            {status?.features?.slice(0, 8).map((feature) => (
              <div key={feature} className="feature-item">
                <span className="feature-icon">✓</span>
                <span className="feature-name">{formatFeatureName(feature)}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

const StatCard = ({ title, value, icon, color }) => (
  <div className={`stat-card stat-${color}`} data-testid={`stat-${title.toLowerCase()}`}>
    <div className="stat-icon">{getIcon(icon)}</div>
    <div className="stat-content">
      <span className="stat-value">{value}</span>
      <span className="stat-title">{title}</span>
    </div>
  </div>
);

const ConnectorStatusItem = ({ connector }) => {
  const statusColor = connector.status === "healthy" || connector.status === "available" ? "green" : 
                      connector.status === "unhealthy" ? "red" : "gray";
  return (
    <div className="connector-item" data-testid={`connector-${connector.type}`}>
      <div className="connector-info">
        <span className="connector-icon">{getIcon(connector.type)}</span>
        <span className="connector-name">{connector.type}</span>
      </div>
      <div className="connector-status">
        <span className={`status-dot status-${statusColor}`}></span>
        <span className="status-text">{connector.status}</span>
      </div>
    </div>
  );
};

const formatFeatureName = (name) => {
  return name.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
};

// ============ JIT Access Page ============

const JITAccess = () => {
  const [grants, setGrants] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showRequest, setShowRequest] = useState(false);

  const fetchGrants = useCallback(async () => {
    try {
      const response = await apiClient.getJITGrants();
      setGrants(response.data.grants || []);
    } catch (err) {
      console.error("Failed to fetch JIT grants", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchGrants();
  }, [fetchGrants]);

  const handleRevoke = async (id) => {
    if (window.confirm("Revoke this access?")) {
      try {
        await apiClient.revokeJIT(id);
        fetchGrants();
      } catch (err) {
        alert(`Failed to revoke: ${err.response?.data?.detail || err.message}`);
      }
    }
  };

  const handleRenew = async (id) => {
    try {
      await apiClient.renewJIT(id);
      fetchGrants();
    } catch (err) {
      alert(`Failed to renew: ${err.response?.data?.detail || err.message}`);
    }
  };

  return (
    <div className="jit-page" data-testid="jit-page">
      <header className="page-header">
        <div>
          <h1>JIT Access</h1>
          <p className="subtitle">Just-In-Time temporary access management (Enterprise)</p>
        </div>
        <button
          className="btn btn-primary"
          onClick={() => setShowRequest(true)}
          data-testid="request-jit-btn"
        >
          + Request Access
        </button>
      </header>

      {loading ? (
        <LoadingSpinner />
      ) : grants.length === 0 ? (
        <EmptyState
          message="No JIT grants found"
          action="Request temporary access to policies"
        />
      ) : (
        <div className="policies-table">
          <table data-testid="jit-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Policy</th>
                <th>Grantee</th>
                <th>Status</th>
                <th>Expires</th>
                <th>Renewals</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {grants.map((grant) => (
                <tr key={grant.id} data-testid={`jit-row-${grant.id}`}>
                  <td><code>{grant.id}</code></td>
                  <td>{grant.policyName}</td>
                  <td>{grant.grantee}</td>
                  <td>
                    <span className={`badge badge-${grant.status}`}>
                      {grant.status}
                    </span>
                  </td>
                  <td>{new Date(grant.expiresAt).toLocaleString()}</td>
                  <td>{grant.renewals}/{grant.maxRenewals}</td>
                  <td className="actions">
                    {grant.status === "active" && (
                      <>
                        <button
                          className="btn btn-sm btn-secondary"
                          onClick={() => handleRenew(grant.id)}
                          disabled={grant.renewals >= grant.maxRenewals}
                          title="Renew"
                        >
                          ↻
                        </button>
                        <button
                          className="btn btn-sm btn-danger"
                          onClick={() => handleRevoke(grant.id)}
                          title="Revoke"
                        >
                          ✗
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showRequest && (
        <JITRequestModal
          onClose={() => setShowRequest(false)}
          onCreated={() => {
            setShowRequest(false);
            fetchGrants();
          }}
        />
      )}
    </div>
  );
};

const JITRequestModal = ({ onClose, onCreated }) => {
  const [policyName, setPolicyName] = useState("");
  const [grantee, setGrantee] = useState("");
  const [reason, setReason] = useState("");
  const [duration, setDuration] = useState("2h");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!policyName || !grantee) {
      alert("Policy and Grantee are required");
      return;
    }
    setSubmitting(true);
    try {
      await apiClient.requestJIT({ policyName, grantee, reason, duration });
      onCreated();
    } catch (err) {
      alert(`Failed: ${err.response?.data?.detail || err.message}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} data-testid="jit-request-modal">
        <div className="modal-header">
          <h2>Request JIT Access</h2>
          <button className="close-btn" onClick={onClose}>✗</button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label>Policy Name *</label>
            <input
              type="text"
              value={policyName}
              onChange={(e) => setPolicyName(e.target.value)}
              placeholder="e.g., marketing-team-access"
              data-testid="jit-policy-input"
            />
          </div>
          <div className="form-group">
            <label>Grantee (User/Email) *</label>
            <input
              type="text"
              value={grantee}
              onChange={(e) => setGrantee(e.target.value)}
              placeholder="e.g., user@company.com"
              data-testid="jit-grantee-input"
            />
          </div>
          <div className="form-group">
            <label>Duration</label>
            <select value={duration} onChange={(e) => setDuration(e.target.value)}>
              <option value="30m">30 minutes</option>
              <option value="1h">1 hour</option>
              <option value="2h">2 hours</option>
              <option value="4h">4 hours</option>
              <option value="8h">8 hours</option>
              <option value="24h">24 hours</option>
            </select>
          </div>
          <div className="form-group">
            <label>Reason</label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Why do you need this access?"
              rows={3}
            />
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button
            className="btn btn-primary"
            onClick={handleSubmit}
            disabled={submitting}
            data-testid="submit-jit-btn"
          >
            {submitting ? "Requesting..." : "Request Access"}
          </button>
        </div>
      </div>
    </div>
  );
};

// ============ Drift Detection Page ============

const DriftDetection = () => {
  const [drifts, setDrifts] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchDrifts = useCallback(async () => {
    try {
      const response = await apiClient.getDrifts();
      setDrifts(response.data.drifts || []);
    } catch (err) {
      console.error("Failed to fetch drifts", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDrifts();
  }, [fetchDrifts]);

  const handleRepair = async (id) => {
    try {
      await apiClient.repairDrift(id);
      fetchDrifts();
    } catch (err) {
      alert(`Failed to repair: ${err.response?.data?.detail || err.message}`);
    }
  };

  const handleRepairAll = async () => {
    try {
      await apiClient.repairAllDrifts();
      fetchDrifts();
    } catch (err) {
      alert(`Failed: ${err.response?.data?.detail || err.message}`);
    }
  };

  const handleSimulate = async () => {
    try {
      await apiClient.simulateDrift("demo-policy");
      fetchDrifts();
    } catch (err) {
      alert(`Failed: ${err.response?.data?.detail || err.message}`);
    }
  };

  const detectedDrifts = drifts.filter(d => d.status === "detected");

  return (
    <div className="drift-page" data-testid="drift-page">
      <header className="page-header">
        <div>
          <h1>Drift Detection</h1>
          <p className="subtitle">Self-Healing infrastructure monitoring (Enterprise)</p>
        </div>
        <div className="header-actions">
          <button className="btn btn-secondary" onClick={handleSimulate}>
            Simulate Drift
          </button>
          {detectedDrifts.length > 0 && (
            <button className="btn btn-primary" onClick={handleRepairAll}>
              Repair All ({detectedDrifts.length})
            </button>
          )}
        </div>
      </header>

      {loading ? (
        <LoadingSpinner />
      ) : drifts.length === 0 ? (
        <div className="success-state">
          <span className="success-icon">✓</span>
          <h3>All Resources In Sync</h3>
          <p>No configuration drift detected</p>
        </div>
      ) : (
        <div className="drift-grid">
          {drifts.map((drift) => (
            <DriftCard key={drift.id} drift={drift} onRepair={handleRepair} />
          ))}
        </div>
      )}
    </div>
  );
};

const DriftCard = ({ drift, onRepair }) => (
  <div className={`drift-card drift-${drift.status}`} data-testid={`drift-card-${drift.id}`}>
    <div className="drift-header">
      <span className="drift-type">{drift.driftType}</span>
      <span className={`drift-status status-${drift.status === "detected" ? "red" : "green"}`}>
        {drift.status}
      </span>
    </div>
    <div className="drift-body">
      <div className="drift-info">
        <label>Policy</label>
        <span>{drift.policyName}</span>
      </div>
      <div className="drift-info">
        <label>Resource</label>
        <span>{drift.resourceType}/{drift.resourceName}</span>
      </div>
      <div className="drift-info">
        <label>Detected</label>
        <span>{new Date(drift.detectedAt).toLocaleString()}</span>
      </div>
      {drift.repairedAt && (
        <div className="drift-info">
          <label>Repaired</label>
          <span>{new Date(drift.repairedAt).toLocaleString()}</span>
        </div>
      )}
    </div>
    {drift.status === "detected" && (
      <div className="drift-footer">
        <button className="btn btn-primary btn-sm" onClick={() => onRepair(drift.id)}>
          Repair Now
        </button>
      </div>
    )}
  </div>
);

// ============ Policies Page (existing, simplified) ============

const Policies = () => {
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);

  const fetchPolicies = useCallback(async () => {
    try {
      const response = await apiClient.getPolicies();
      setPolicies(response.data.policies || []);
    } catch (err) {
      console.error("Failed to fetch policies", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchPolicies(); }, [fetchPolicies]);

  return (
    <div className="policies-page" data-testid="policies-page">
      <header className="page-header">
        <div>
          <h1>Policies</h1>
          <p className="subtitle">Manage access policies</p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowCreate(true)} data-testid="create-policy-btn">
          + Create Policy
        </button>
      </header>

      {loading ? <LoadingSpinner /> : policies.length === 0 ? (
        <EmptyState message="No policies found" action="Create your first policy" />
      ) : (
        <div className="policies-table">
          <table data-testid="policies-table">
            <thead>
              <tr><th>Name</th><th>Kind</th><th>Namespace</th><th>Team</th><th>Targets</th><th>Status</th></tr>
            </thead>
            <tbody>
              {policies.map((policy) => (
                <tr key={policy.name}>
                  <td>{policy.name}</td>
                  <td><span className="badge badge-kind">{policy.kind}</span></td>
                  <td>{policy.namespace || "-"}</td>
                  <td>{policy.team || "-"}</td>
                  <td>{policy.targets}</td>
                  <td><span className={`badge badge-${policy.status?.toLowerCase()}`}>{policy.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && <CreatePolicyModal onClose={() => setShowCreate(false)} onCreated={() => { setShowCreate(false); fetchPolicies(); }} />}
    </div>
  );
};

const CreatePolicyModal = ({ onClose, onCreated }) => {
  const [yaml, setYaml] = useState(defaultPolicyYAML);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      await apiClient.createPolicy({ yaml });
      onCreated();
    } catch (err) {
      alert(`Failed: ${err.response?.data?.detail || err.message}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} data-testid="create-policy-modal">
        <div className="modal-header">
          <h2>Create Policy</h2>
          <button className="close-btn" onClick={onClose}>✗</button>
        </div>
        <div className="modal-body">
          <textarea className="yaml-editor" value={yaml} onChange={(e) => setYaml(e.target.value)} rows={20} data-testid="policy-yaml-input" />
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSubmit} disabled={submitting} data-testid="submit-policy-btn">
            {submitting ? "Creating..." : "Create Policy"}
          </button>
        </div>
      </div>
    </div>
  );
};

const defaultPolicyYAML = `apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: my-team-access
  namespace: production
spec:
  team: platform
  members:
    - user@company.com
  targets:
    - type: kubernetes
      name: k8s-prod
      kubernetes:
        namespace: production
        resources: [pods, services]
        verbs: [get, list]`;

// ============ Other Pages ============

const Connectors = () => {
  const [connectors, setConnectors] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiClient.getConnectors().then(r => { setConnectors(r.data.connectors || []); setLoading(false); }).catch(() => setLoading(false));
  }, []);

  if (loading) return <LoadingSpinner />;

  return (
    <div className="connectors-page" data-testid="connectors-page">
      <header className="page-header"><h1>Connectors</h1><p className="subtitle">Infrastructure connectors</p></header>
      <div className="connectors-grid">
        {connectors.map((conn) => <ConnectorCard key={conn.type} connector={conn} />)}
      </div>
    </div>
  );
};

const ConnectorCard = ({ connector }) => {
  const icons = { kubernetes: "☸", kafka: "◈", kong: "◆" };
  const statusColor = connector.status === "healthy" || connector.status === "available" ? "green" : connector.status === "unhealthy" ? "red" : "gray";
  return (
    <div className={`connector-card connector-${connector.type}`} data-testid={`connector-card-${connector.type}`}>
      <div className="connector-header">
        <span className="connector-icon-lg">{icons[connector.type] || "•"}</span>
        <h3>{connector.type}</h3>
      </div>
      <div className="connector-body">
        <div className="connector-detail"><label>Status</label><span className={`status-badge status-${statusColor}`}>{connector.status}</span></div>
        <div className="connector-detail"><label>Mode</label><span>{connector.mode || "not configured"}</span></div>
      </div>
      <div className="connector-footer">
        <span className={`feature-tag ${connector.type === "kubernetes" ? "community" : "enterprise"}`}>
          {connector.type === "kubernetes" ? "Community" : "Enterprise"}
        </span>
      </div>
    </div>
  );
};

const AuditLogs = () => {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiClient.getAuditLogs().then(r => { setLogs(r.data.logs || []); setLoading(false); }).catch(() => setLoading(false));
  }, []);

  if (loading) return <LoadingSpinner />;

  return (
    <div className="audit-page" data-testid="audit-page">
      <header className="page-header"><h1>Audit Logs</h1><p className="subtitle">Unified audit trail</p></header>
      {logs.length === 0 ? <EmptyState message="No audit logs" action="Actions will be logged here" /> : (
        <div className="policies-table">
          <table>
            <thead><tr><th>Timestamp</th><th>Action</th><th>Policy</th><th>Actor</th><th>Details</th></tr></thead>
            <tbody>
              {logs.map((log) => (
                <tr key={log.id}><td>{log.timestamp}</td><td><span className="badge badge-kind">{log.action}</span></td><td>{log.policy}</td><td>{log.actor}</td><td>{log.details}</td></tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

const Settings = () => {
  const [license, setLicense] = useState(null);
  const [licenseKey, setLicenseKey] = useState("");

  useEffect(() => { apiClient.getLicense().then(r => setLicense(r.data)).catch(() => {}); }, []);

  const handleUpdateLicense = async () => {
    try {
      const r = await apiClient.setLicense(licenseKey);
      setLicense(r.data);
      setLicenseKey("");
      alert("License updated!");
    } catch (err) {
      alert(`Failed: ${err.message}`);
    }
  };

  return (
    <div className="settings-page" data-testid="settings-page">
      <header className="page-header"><h1>Settings</h1><p className="subtitle">Configure Gatewise</p></header>
      <div className="settings-section">
        <h2>License</h2>
        <div className="card">
          <div className="license-info">
            <div className="license-tier"><label>Tier</label><span className={`tier-badge tier-${license?.tier}`}>{license?.tier || "community"}</span></div>
            <div className="license-features"><label>Features</label><div className="feature-tags">{license?.features?.map(f => <span key={f} className="feature-tag">{formatFeatureName(f)}</span>)}</div></div>
          </div>
          <div className="license-input">
            <input type="text" placeholder="Enter Enterprise key (ENT-...)" value={licenseKey} onChange={(e) => setLicenseKey(e.target.value)} data-testid="license-key-input" />
            <button className="btn btn-primary" onClick={handleUpdateLicense} disabled={!licenseKey} data-testid="update-license-btn">Update</button>
          </div>
        </div>
      </div>
    </div>
  );
};

// ============ Common Components ============

const LoadingSpinner = () => (<div className="loading-spinner" data-testid="loading-spinner"><div className="spinner"></div><span>Loading...</span></div>);
const ErrorMessage = ({ message }) => (<div className="error-message" data-testid="error-message"><span className="error-icon">⚠</span><span>{message}</span></div>);
const EmptyState = ({ message, action }) => (<div className="empty-state" data-testid="empty-state"><span className="empty-icon">📭</span><h3>{message}</h3><p>{action}</p></div>);

// ============ Main App ============

function App() {
  const [realtimeData, setRealtimeData] = useState({});
  const [wsConnected, setWsConnected] = useState(false);

  const handleWSMessage = useCallback((data) => {
    console.log("WS message:", data);
    if (data.type === "connected") {
      setRealtimeData(data.data);
      setWsConnected(true);
    } else if (data.type === "drift_detected") {
      setRealtimeData(prev => ({ ...prev, driftsDetected: (prev.driftsDetected || 0) + 1 }));
    } else if (data.type === "jit_granted") {
      setRealtimeData(prev => ({ ...prev, jitActive: (prev.jitActive || 0) + 1 }));
    }
  }, []);

  useWebSocket(handleWSMessage);

  return (
    <div className="app-container">
      <BrowserRouter>
        <Sidebar wsConnected={wsConnected} />
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Dashboard realtimeData={realtimeData} />} />
            <Route path="/policies" element={<Policies />} />
            <Route path="/connectors" element={<Connectors />} />
            <Route path="/jit" element={<JITAccess />} />
            <Route path="/drift" element={<DriftDetection />} />
            <Route path="/audit" element={<AuditLogs />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </BrowserRouter>
    </div>
  );
}

export default App;
