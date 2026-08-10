import axios from 'axios';

// Base URL for all API calls
// In production on EKS this points to the ALB
// Locally this points to localhost
const BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost';

// Auth Service calls
export const authAPI = {
  register: async (name, email, password) => {
    const response = await axios.post(`${BASE_URL}:8081/api/auth/register`, {
      name,
      email,
      password,
    });
    return response.data;
  },

  login: async (email, password) => {
    const response = await axios.post(`${BASE_URL}:8081/api/auth/login`, {
      email,
      password,
    });
    return response.data;
  },
};

// Orders Service calls
export const ordersAPI = {
  createOrder: async (token, itemName, quantity, price) => {
    const response = await axios.post(
      `${BASE_URL}:8082/api/orders`,
      { item_name: itemName, quantity, price },
      { headers: { Authorization: `Bearer ${token}` } }
    );
    return response.data;
  },

  getOrders: async (token) => {
    const response = await axios.get(`${BASE_URL}:8082/api/orders`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    return response.data;
  },
};

// Worker Service calls
export const workerAPI = {
  startSimulation: async () => {
    const response = await axios.post(
      `${BASE_URL}:8084/api/worker/simulate/start`
    );
    return response.data;
  },

  stopSimulation: async () => {
    const response = await axios.post(
      `${BASE_URL}:8084/api/worker/simulate/stop`
    );
    return response.data;
  },

  getStatus: async () => {
    const response = await axios.get(
      `${BASE_URL}:8084/api/worker/simulate/status`
    );
    return response.data;
  },
};