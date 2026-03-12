import { useState, useEffect, useCallback } from "react";
import "@/App.css";
import { BrowserRouter, Routes, Route, Link, useLocation } from "react-router-dom";
import axios from "axios";

const BACKEND_URL = process.env.REACT_APP_BACKEND_URL;
const API = `${BACKEND_URL}/api`;

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
};

// ============ Components ============

const Sidebar = () => {
  const location = useLocation();
  
  const links = [
    { path: "/", label: "Dashboard", icon: "grid" },
    { path: "/policies", label: "Policies", icon: "file-text" },
    { path: "/connectors", label: "Connectors", icon: "plug" },
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
        <span className="version">v0.2.0</span>
      </div>
      <nav className="sidebar-nav">
        {links.map((link) => (
          <Link
            key={link.path}
            to={link.path}
            className={`nav-link ${location.pathname === link.path ? "active" : ""}`}
            data-testid={`nav-${link.label.toLowerCase()}`}
          >
            <span className="nav-icon">{getIcon(link.icon)}</span>
            <span className="nav-label">{link.label}</span>
          </Link>
        ))}
      </nav>
      <div className="sidebar-footer">
        <div className="license-badge" data-testid="license-badge">
          <span>Community</span>
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
    kubernetes: "☸",
    kafka: "◈",
    kong: "◆",
  };
  return icons[name] || "•";
};

// ============ Dashboard Page ============

const Dashboard = () => {
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
          title="License"
          value={status?.license || "community"}
          icon="check"
          color="green"
        />
        <StatCard
          title="Database"
          value={status?.databaseOk ? "Connected" : "Offline"}
          icon={status?.databaseOk ? "check" : "x"}
          color={status?.databaseOk ? "green" : "red"}
        />
        <StatCard
          title="Version"
          value={status?.version || "0.2.0"}
          icon="settings"
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
            {status?.features?.map((feature) => (
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
  const statusColor = connector.status === "healthy" ? "green" : 
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

// ============ Policies Page ============

const Policies = () => {
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [selectedPolicy, setSelectedPolicy] = useState(null);

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

  useEffect(() => {
    fetchPolicies();
  }, [fetchPolicies]);

  const handleReconcile = async (name) => {
    try {
      await apiClient.reconcilePolicy(name);
      fetchPolicies();
    } catch (err) {
      alert(`Failed to reconcile: ${err.message}`);
    }
  };

  const handleDelete = async (name) => {
    if (window.confirm(`Delete policy "${name}"?`)) {
      try {
        await apiClient.deletePolicy(name);
        fetchPolicies();
      } catch (err) {
        alert(`Failed to delete: ${err.message}`);
      }
    }
  };

  return (
    <div className="policies-page" data-testid="policies-page">
      <header className="page-header">
        <div>
          <h1>Policies</h1>
          <p className="subtitle">Manage access policies for your infrastructure</p>
        </div>
        <button
          className="btn btn-primary"
          onClick={() => setShowCreate(true)}
          data-testid="create-policy-btn"
        >
          + Create Policy
        </button>
      </header>

      {loading ? (
        <LoadingSpinner />
      ) : policies.length === 0 ? (
        <EmptyState
          message="No policies found"
          action="Create your first policy to get started"
        />
      ) : (
        <div className="policies-table">
          <table data-testid="policies-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Kind</th>
                <th>Namespace</th>
                <th>Team</th>
                <th>Targets</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {policies.map((policy) => (
                <tr key={policy.name} data-testid={`policy-row-${policy.name}`}>
                  <td>
                    <button
                      className="link-btn"
                      onClick={() => setSelectedPolicy(policy)}
                    >
                      {policy.name}
                    </button>
                  </td>
                  <td><span className="badge badge-kind">{policy.kind}</span></td>
                  <td>{policy.namespace || "-"}</td>
                  <td>{policy.team || "-"}</td>
                  <td>{policy.targets}</td>
                  <td>
                    <span className={`badge badge-${policy.status?.toLowerCase()}`}>
                      {policy.status}
                    </span>
                  </td>
                  <td className="actions">
                    <button
                      className="btn btn-sm btn-secondary"
                      onClick={() => handleReconcile(policy.name)}
                      title="Reconcile"
                    >
                      ↻
                    </button>
                    <button
                      className="btn btn-sm btn-danger"
                      onClick={() => handleDelete(policy.name)}
                      title="Delete"
                    >
                      ✗
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreatePolicyModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            fetchPolicies();
          }}
        />
      )}

      {selectedPolicy && (
        <PolicyDetailModal
          policy={selectedPolicy}
          onClose={() => setSelectedPolicy(null)}
        />
      )}
    </div>
  );
};

const CreatePolicyModal = ({ onClose, onCreated }) => {
  const [yaml, setYaml] = useState(defaultPolicyYAML);
  const [validation, setValidation] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const handleValidate = async () => {
    try {
      const response = await apiClient.validatePolicy(yaml);
      setValidation(response.data);
    } catch (err) {
      setValidation({ valid: false, errors: [{ message: err.message }] });
    }
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      await apiClient.createPolicy({ yaml });
      onCreated();
    } catch (err) {
      alert(`Failed to create policy: ${err.response?.data?.error || err.message}`);
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
          <div className="form-group">
            <label>Policy YAML</label>
            <textarea
              className="yaml-editor"
              value={yaml}
              onChange={(e) => setYaml(e.target.value)}
              rows={20}
              data-testid="policy-yaml-input"
            />
          </div>
          {validation && (
            <div className={`validation-result ${validation.valid ? "valid" : "invalid"}`}>
              {validation.valid ? (
                <span>✓ Policy is valid</span>
              ) : (
                <div>
                  <span>✗ Validation errors:</span>
                  <ul>
                    {validation.errors?.map((err, i) => (
                      <li key={i}>{err.field}: {err.message}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={handleValidate}>
            Validate
          </button>
          <button
            className="btn btn-primary"
            onClick={handleSubmit}
            disabled={submitting}
            data-testid="submit-policy-btn"
          >
            {submitting ? "Creating..." : "Create Policy"}
          </button>
        </div>
      </div>
    </div>
  );
};

const PolicyDetailModal = ({ policy, onClose }) => (
  <div className="modal-overlay" onClick={onClose}>
    <div className="modal modal-lg" onClick={(e) => e.stopPropagation()}>
      <div className="modal-header">
        <h2>{policy.name}</h2>
        <button className="close-btn" onClick={onClose}>✗</button>
      </div>
      <div className="modal-body">
        <div className="policy-detail-grid">
          <div className="detail-item">
            <label>Kind</label>
            <span>{policy.kind}</span>
          </div>
          <div className="detail-item">
            <label>Namespace</label>
            <span>{policy.namespace || "-"}</span>
          </div>
          <div className="detail-item">
            <label>Team</label>
            <span>{policy.team || "-"}</span>
          </div>
          <div className="detail-item">
            <label>Targets</label>
            <span>{policy.targets}</span>
          </div>
          <div className="detail-item">
            <label>Status</label>
            <span className={`badge badge-${policy.status?.toLowerCase()}`}>{policy.status}</span>
          </div>
          <div className="detail-item">
            <label>Created</label>
            <span>{new Date(policy.createdAt).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
);

const defaultPolicyYAML = `apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: my-team-access
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
          - list
`;

// ============ Connectors Page ============

const Connectors = () => {
  const [connectors, setConnectors] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchConnectors = async () => {
      try {
        const response = await apiClient.getConnectors();
        setConnectors(response.data.connectors || []);
      } catch (err) {
        console.error("Failed to fetch connectors", err);
      } finally {
        setLoading(false);
      }
    };
    fetchConnectors();
  }, []);

  if (loading) return <LoadingSpinner />;

  return (
    <div className="connectors-page" data-testid="connectors-page">
      <header className="page-header">
        <h1>Connectors</h1>
        <p className="subtitle">Infrastructure connectors for K8s, Kafka, and Kong</p>
      </header>

      <div className="connectors-grid">
        {connectors.map((conn) => (
          <ConnectorCard key={conn.type} connector={conn} />
        ))}
      </div>
    </div>
  );
};

const ConnectorCard = ({ connector }) => {
  const icons = {
    kubernetes: "☸",
    kafka: "◈",
    kong: "◆",
  };

  const statusColor = connector.status === "healthy" ? "green" :
                      connector.status === "unhealthy" ? "red" : "gray";

  return (
    <div className={`connector-card connector-${connector.type}`} data-testid={`connector-card-${connector.type}`}>
      <div className="connector-header">
        <span className="connector-icon-lg">{icons[connector.type] || "•"}</span>
        <h3>{connector.type}</h3>
      </div>
      <div className="connector-body">
        <div className="connector-detail">
          <label>Status</label>
          <span className={`status-badge status-${statusColor}`}>
            {connector.status}
          </span>
        </div>
        <div className="connector-detail">
          <label>Mode</label>
          <span>{connector.mode || "not configured"}</span>
        </div>
        <div className="connector-detail">
          <label>Enabled</label>
          <span>{connector.enabled ? "Yes" : "No"}</span>
        </div>
      </div>
      <div className="connector-footer">
        {connector.type === "kubernetes" ? (
          <span className="feature-tag community">Community</span>
        ) : (
          <span className="feature-tag enterprise">Enterprise</span>
        )}
      </div>
    </div>
  );
};

// ============ Audit Page ============

const AuditLogs = () => (
  <div className="audit-page" data-testid="audit-page">
    <header className="page-header">
      <h1>Audit Logs</h1>
      <p className="subtitle">Unified audit trail for K8s, Kafka, and Kong</p>
    </header>
    <EmptyState
      message="Audit logs will appear here"
      action="Actions on policies will be logged automatically"
    />
  </div>
);

// ============ Settings Page ============

const Settings = () => {
  const [license, setLicense] = useState(null);
  const [licenseKey, setLicenseKey] = useState("");

  useEffect(() => {
    const fetchLicense = async () => {
      try {
        const response = await apiClient.getLicense();
        setLicense(response.data);
      } catch (err) {
        console.error("Failed to fetch license", err);
      }
    };
    fetchLicense();
  }, []);

  const handleUpdateLicense = async () => {
    try {
      const response = await apiClient.setLicense(licenseKey);
      setLicense(response.data);
      setLicenseKey("");
      alert("License updated successfully!");
    } catch (err) {
      alert(`Failed to update license: ${err.message}`);
    }
  };

  return (
    <div className="settings-page" data-testid="settings-page">
      <header className="page-header">
        <h1>Settings</h1>
        <p className="subtitle">Configure Gatewise Control Plane</p>
      </header>

      <div className="settings-section">
        <h2>License</h2>
        <div className="card">
          <div className="license-info">
            <div className="license-tier">
              <label>Current Tier</label>
              <span className={`tier-badge tier-${license?.tier}`}>
                {license?.tier || "community"}
              </span>
            </div>
            <div className="license-features">
              <label>Enabled Features</label>
              <div className="feature-tags">
                {license?.features?.map((f) => (
                  <span key={f} className="feature-tag">{formatFeatureName(f)}</span>
                ))}
              </div>
            </div>
          </div>
          <div className="license-input">
            <input
              type="text"
              placeholder="Enter Enterprise license key (ENT-...)"
              value={licenseKey}
              onChange={(e) => setLicenseKey(e.target.value)}
              data-testid="license-key-input"
            />
            <button
              className="btn btn-primary"
              onClick={handleUpdateLicense}
              disabled={!licenseKey}
              data-testid="update-license-btn"
            >
              Update License
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

// ============ Common Components ============

const LoadingSpinner = () => (
  <div className="loading-spinner" data-testid="loading-spinner">
    <div className="spinner"></div>
    <span>Loading...</span>
  </div>
);

const ErrorMessage = ({ message }) => (
  <div className="error-message" data-testid="error-message">
    <span className="error-icon">⚠</span>
    <span>{message}</span>
  </div>
);

const EmptyState = ({ message, action }) => (
  <div className="empty-state" data-testid="empty-state">
    <span className="empty-icon">📭</span>
    <h3>{message}</h3>
    <p>{action}</p>
  </div>
);

// ============ Main App ============

function App() {
  return (
    <div className="app-container">
      <BrowserRouter>
        <Sidebar />
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/policies" element={<Policies />} />
            <Route path="/connectors" element={<Connectors />} />
            <Route path="/audit" element={<AuditLogs />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </BrowserRouter>
    </div>
  );
}

export default App;
