const API_BASE = '/api/v1';
let token = localStorage.getItem('civora_token') || '';
let orgId = localStorage.getItem('civora_org_id') || '';

function setAuth(t, o) {
  token = t;
  orgId = o;
  localStorage.setItem('civora_token', t);
  localStorage.setItem('civora_org_id', o);
}

function clearAuth() {
  token = '';
  orgId = '';
  localStorage.removeItem('civora_token');
  localStorage.removeItem('civora_org_id');
}

async function api(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) { clearAuth(); router.navigate('login'); }
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error?.message || `HTTP ${res.status}`);
  }
  return res.json().catch(() => ({}));
}
