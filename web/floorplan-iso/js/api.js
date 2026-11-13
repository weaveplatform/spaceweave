// Client for the API that `spaceweave serve` adds to the site
// (internal/server). Under a plain static server such as tools/serve.py,
// info() fails and the page leaves uploads out.

// Must match RequestHeader in internal/server. A page on another origin
// can't send it without a CORS preflight, which the server never approves.
export const REQUEST_HEADER = 'X-Spaceweave';

export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

export function createApi(fetchImpl = globalThis.fetch) {
  async function call(path, init = {}) {
    const headers = init.method === 'POST' ? { [REQUEST_HEADER]: '1', ...init.headers } : init.headers;
    const res = await fetchImpl(`api/${path}`, { cache: 'no-store', ...init, headers });
    let body = {};
    try {
      body = await res.json();
    } catch { /* not JSON: a static server's 404 page */ }
    if (!res.ok) throw new ApiError(body.error || `HTTP ${res.status}`, res.status);
    return body;
  }
  return {
    info: () => call('info'),
    upload(file, name = file.name) {
      const form = new FormData();
      form.append('image', file, name);
      return call('plans', { method: 'POST', body: form });
    },
    review: (id, action, note = '') => call(`plans/${encodeURIComponent(id)}/${action}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ note }),
    }),
  };
}
