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
    throw new Error(`${res.status} ${err.error?.message || `HTTP ${res.status}`}`);
  }
  return res.json().catch(() => ({}));
}

function decodeJWT(t) {
  try {
    const parts = t.split('.');
    if (parts.length < 2) return null;
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(b64));
  } catch (e) {
    return null;
  }
}

function getCurrentUser() {
  if (!token) return null;
  const claims = decodeJWT(token);
  if (!claims) return null;
  return { id: claims.sub, organization_id: claims.organization_id, role: claims.role };
}

function currentUserIsAdmin() {
  const u = getCurrentUser();
  return !!(u && u.role === 'admin');
}
