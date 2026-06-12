import axios from 'axios';

const client = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://192.168.10.100:9000',
  timeout: 300000, // 5 分钟，支持 Browser Agent 等长时间操作
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  const tenantId = localStorage.getItem('tenantId') || 'default';
  config.headers['X-Tenant-ID'] = tenantId;
  return config;
});

// 响应拦截器
client.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      // 未授权处理
      localStorage.removeItem('token');
    }
    return Promise.reject(error);
  }
);

export default client;