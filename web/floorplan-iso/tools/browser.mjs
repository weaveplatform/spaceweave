// Shared CDP connection. Callers own and close their own browser targets.
export async function connect(url) {
  const ws = new WebSocket(url), pending = new Map(), errors = [], handlers = new Map();
  let serial = 0;
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', reject, { once: true });
  });
  ws.addEventListener('message', e => {
    const message = JSON.parse(e.data);
    if (message.id) {
      const p = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) p.reject(Error(JSON.stringify(message.error)));
      else p.resolve(message.result);
    } else {
      if(message.method==='Runtime.exceptionThrown')errors.push(message.params.exceptionDetails.exception?.description??message.params.exceptionDetails.text);
      for(const handler of handlers.get(message.method)??[])Promise.resolve(handler(message.params)).catch(err=>errors.push(err.message));
    }
  });
  return {
    call: (method, params = {}) => new Promise((resolve, reject) => {
      const id = ++serial;
      pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    }),
    on(method, handler) { if(!handlers.has(method))handlers.set(method,new Set());handlers.get(method).add(handler);return()=>handlers.get(method).delete(handler); },
    close: () => ws.close(), errors,
  };
}
