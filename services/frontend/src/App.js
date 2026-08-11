// ShopSRE Frontend v1.0.0
import React, { useState, useEffect } from 'react';
import { authAPI, ordersAPI, workerAPI } from './api/client';
import './App.css';

function App() {
  // Auth state
  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [userInfo, setUserInfo] = useState(null);

  // Response display
  const [response, setResponse] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);
  const [activeButton, setActiveButton] = useState(null);

  // Simulator state
  const [simStatus, setSimStatus] = useState(null);
  const [simRunning, setSimRunning] = useState(false);

  // Poll simulator status every second when running
  useEffect(() => {
    let interval;
    if (simRunning) {
      interval = setInterval(async () => {
        try {
          const status = await workerAPI.getStatus();
          setSimStatus(status);
          if (!status.is_running) {
            setSimRunning(false);
          }
        } catch (err) {
          // Pod was OOM killed — stop polling
          setSimRunning(false);
          setSimStatus(null);
          setError('Pod was OOM killed! Check Grafana and X-Ray.');
        }
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [simRunning]);

  const handleAction = async (buttonName, action) => {
    setLoading(true);
    setActiveButton(buttonName);
    setError(null);
    setResponse(null);

    const startTime = Date.now();

    try {
      const result = await action();
      const duration = Date.now() - startTime;
      setResponse({ ...result, duration_ms: duration });
    } catch (err) {
      const duration = Date.now() - startTime;
      setError({
        message: err.response?.data?.message || err.message,
        status: err.response?.status,
        duration_ms: duration,
      });
    } finally {
      setLoading(false);
    }
  };

  // Button 1 — Login
  const handleLogin = () =>
    handleAction('login', async () => {
      const result = await authAPI.login('demo@shopsre.com', 'demo123');
      setToken(result.token);
      localStorage.setItem('token', result.token);
      setUserInfo({ name: result.name, email: result.email });
      return result;
    });

  // Register demo user first if login fails
  const handleRegister = () =>
    handleAction('register', async () => {
      const result = await authAPI.register(
        'Demo User',
        'demo@shopsre.com',
        'demo123'
      );
      return result;
    });

  // Button 2 — Place Order
  const handlePlaceOrder = () =>
    handleAction('place-order', async () => {
      if (!token) throw new Error('Please login first');
      return await ordersAPI.createOrder(
        token,
        'MacBook Pro M3',
        1,
        2499.99
      );
    });

  // Button 3 — Get Orders
  const handleGetOrders = () =>
    handleAction('get-orders', async () => {
      if (!token) throw new Error('Please login first');
      return await ordersAPI.getOrders(token);
    });

  // Button 4 — Simulate Load
  const handleSimulate = () =>
    handleAction('simulate', async () => {
      const result = await workerAPI.startSimulation();
      setSimRunning(true);
      return result;
    });

  const handleStopSimulate = () =>
    handleAction('stop-simulate', async () => {
      const result = await workerAPI.stopSimulation();
      setSimRunning(false);
      setSimStatus(null);
      return result;
    });

  return (
    <div className="app">
      {/* Header */}
      <header className="header">
        <h1>ShopSRE Platform</h1>
        <p className="subtitle">
          Production-grade microservices with full observability
        </p>
        {userInfo && (
          <div className="user-badge">
            Logged in as {userInfo.name}
          </div>
        )}
      </header>

      {/* Architecture info */}
      <div className="arch-bar">
        <span className="arch-item">Auth Service :8081</span>
        <span className="arch-arrow">→</span>
        <span className="arch-item">Orders Service :8082</span>
        <span className="arch-arrow">→</span>
        <span className="arch-item">Notifications :8083</span>
        <span className="arch-arrow">→</span>
        <span className="arch-item">Worker :8084</span>
      </div>

      {/* Buttons */}
      <div className="buttons-grid">

        {/* Button 1 — Login */}
        <div className="button-card">
          <div className="button-number">1</div>
          <h3>Login</h3>
          <p className="button-desc">
            Calls Auth Service → PostgreSQL → Redis session
          </p>
          <div className="trace-path">
            Frontend → Auth → PostgreSQL → Redis
          </div>
          <button
            className={`action-btn btn-blue ${activeButton === 'login' && loading ? 'loading' : ''}`}
            onClick={handleLogin}
            disabled={loading}
          >
            {loading && activeButton === 'login' ? 'Calling Auth Service...' : 'Login'}
          </button>
          <button
            className="action-btn btn-outline"
            onClick={handleRegister}
            disabled={loading}
          >
            Register Demo User
          </button>
        </div>

        {/* Button 2 — Place Order */}
        <div className="button-card">
          <div className="button-number">2</div>
          <h3>Place Order</h3>
          <p className="button-desc">
            Calls Orders Service → validates JWT → saves to PostgreSQL → publishes to RabbitMQ
          </p>
          <div className="trace-path">
            Frontend → Orders → Auth → PostgreSQL → RabbitMQ → Notifications
          </div>
          <button
            className={`action-btn btn-green ${activeButton === 'place-order' && loading ? 'loading' : ''}`}
            onClick={handlePlaceOrder}
            disabled={loading}
          >
            {loading && activeButton === 'place-order'
              ? 'Placing Order...'
              : 'Place Order'}
          </button>
        </div>

        {/* Button 3 — Get Orders */}
        <div className="button-card">
          <div className="button-number">3</div>
          <h3>Get My Orders</h3>
          <p className="button-desc">
            Calls Orders Service → checks Redis cache → falls back to PostgreSQL
          </p>
          <div className="trace-path">
            Frontend → Orders → Redis (cache hit/miss) → PostgreSQL
          </div>
          <button
            className={`action-btn btn-purple ${activeButton === 'get-orders' && loading ? 'loading' : ''}`}
            onClick={handleGetOrders}
            disabled={loading}
          >
            {loading && activeButton === 'get-orders'
              ? 'Fetching Orders...'
              : 'Get My Orders'}
          </button>
        </div>

        {/* Button 4 — Simulate Load */}
        <div className="button-card button-card-danger">
          <div className="button-number">4</div>
          <h3>Simulate OOM</h3>
          <p className="button-desc">
            Triggers intentional memory leak in Worker Service → Kubernetes OOM kills the pod
          </p>
          <div className="trace-path">
            Frontend → Worker → memory leak → OOM kill → Prometheus alert → Grafana
          </div>

          {/* Memory status bar */}
          {simStatus && (
            <div className="memory-bar-wrap">
              <div className="memory-label">
                Memory: {simStatus.allocated_mb}MB allocated
              </div>
              <div className="memory-bar">
                <div
                  className="memory-fill"
                  style={{
                    width: `${Math.min((simStatus.allocated_mb / 256) * 100, 100)}%`,
                    background: simStatus.allocated_mb > 200 ? '#ef4444' : '#f59e0b',
                  }}
                />
              </div>
              <div className="memory-limit">256MB limit</div>
            </div>
          )}

          {!simRunning ? (
            <button
              className={`action-btn btn-red ${activeButton === 'simulate' && loading ? 'loading' : ''}`}
              onClick={handleSimulate}
              disabled={loading}
            >
              {loading && activeButton === 'simulate'
                ? 'Starting...'
                : 'Start Memory Leak'}
            </button>
          ) : (
            <button
              className="action-btn btn-outline"
              onClick={handleStopSimulate}
              disabled={loading}
            >
              Stop Simulation
            </button>
          )}
        </div>
      </div>

      {/* Response panel */}
      {(response || error) && (
        <div className="response-panel">
          <div className="response-header">
            <span className={`status-badge ${error ? 'badge-error' : 'badge-success'}`}>
              {error ? 'Error' : 'Success'}
            </span>
            <span className="duration">
              {(response || error).duration_ms}ms
            </span>
          </div>

          {error && (
            <div className="error-box">
              <p>{error.message}</p>
              {error.status && <p>HTTP Status: {error.status}</p>}
            </div>
          )}

          {response && (
            <pre className="response-json">
              {JSON.stringify(response, null, 2)}
            </pre>
          )}

          <div className="observability-links">
            <p className="obs-title">Check observability:</p>
            <a href="http://localhost:3001" target="_blank" rel="noreferrer">
              Grafana Dashboard
            </a>
            <a href="http://localhost:16686" target="_blank" rel="noreferrer">
              Jaeger Traces
            </a>
            <a href="http://localhost:9090" target="_blank" rel="noreferrer">
              Prometheus
            </a>
          </div>
        </div>
      )}

      {/* Footer */}
      <footer className="footer">
        <p>
          ShopSRE Platform — EKS · Prometheus · Grafana · AWS X-Ray · CloudWatch
        </p>
      </footer>
    </div>
  );
}

export default App;