const API_URL = '';

async function loadMetrics(days) {
  const response = await fetch(`${API_URL}/v1/admin/metrics?days=${days}`);
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.message || `failed to load metrics (${response.status})`);
  }
  return response.json();
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function renderMetrics(data) {
  const body = document.getElementById('metrics-body');
  if (!body) return;
  body.innerHTML = data.daily
    .map(
      (m) => `
        <tr class="border-b border-slate-100">
          <td class="px-4 py-3 text-slate-600">${escapeHtml(m.date)}</td>
          <td class="px-4 py-3 text-slate-800">${escapeHtml(m.event_name)}</td>
          <td class="px-4 py-3 text-slate-800">${m.called}</td>
          <td class="px-4 py-3 text-slate-800">${m.confirmed}</td>
          <td class="px-4 py-3 text-slate-600">${m.unconfirmed}</td>
          <td class="px-4 py-3 text-slate-600">${m.avg_confirm_minutes}</td>
          <td class="px-4 py-3 text-slate-600">${m.manual_count}</td>
        </tr>`,
    )
    .join('');
  if (data.daily.length === 0) {
    body.innerHTML = '<tr><td colspan="7" class="px-4 py-8 text-center text-slate-500">No data yet.</td></tr>';
  }
}

async function main() {
  const statusEl = document.getElementById('metrics-error');
  const daysEl = document.getElementById('metrics-days');
  const load = async () => {
    try {
      const data = await loadMetrics(daysEl ? daysEl.value : 14);
      renderMetrics(data);
      if (statusEl) statusEl.textContent = '';
    } catch (error) {
      if (statusEl) statusEl.textContent = error.message;
    }
  };
  if (daysEl) daysEl.addEventListener('change', load);
  await load();
}

document.addEventListener('DOMContentLoaded', main);

window.__test = { renderMetrics, loadMetrics };