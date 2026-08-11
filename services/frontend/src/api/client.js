import axios from 'axios';

// ALB URL — routes to correct service based on path
const BASE_URL = process.env.REACT_APP_API_URL || 
  'http://k8s-producti-shopsrei-983d6c7a3d-924921885.ap-south-1.elb.amazonaws.com';

// Auth Service calls — routed via /api/auth path
export const authAPI = {
  register: async (name, email, password) => {
    const response = await axios.post(`${BASE_URL}/api/auth/register`, {
      name,
      email,
      password,
    });
    return response.data;
  },

  login: async (email, password) => {
    const response = await axios.post(`${BASE_URL}/api/auth/login`, {
      email,
      password,
    });
    return response.data;
  },
};

// Orders Service calls — routed via /api/orders path
export const ordersAPI = {
  createOrder: async (token, itemName, quantity, price) => {
    const response = await axios.post(
      `${BASE_URL}/api/orders`,
      { item_name: itemName, quantity, price },
      { headers: { Authorization: `Bearer ${token}` } }
    );
    return response.data;
  },

  getOrders: async (token) => {
    const response = await axios.get(`${BASE_URL}/api/orders`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    return response.data;
  },
};

// Worker Service calls — routed via /api/worker path
export const workerAPI = {
  startSimulation: async () => {
    const response = await axios.post(
      `${BASE_URL}/api/worker/simulate/start`
    );
    return response.data;
  },

  stopSimulation: async () => {
    const response = await axios.post(
      `${BASE_URL}/api/worker/simulate/stop`
    );
    return response.data;
  },

  getStatus: async () => {
    const response = await axios.get(
      `${BASE_URL}/api/worker/simulate/status`
    );
    return response.data;
  },
};