const API_URL = '';

const pageStatus = document.getElementById('page-status');
const eventsBody = document.getElementById('events-body');

let events = [];
let eventCheckWindows = new Map();
let pendingRequests = 0;

function setPageStatus(message, tone = 'info') {
    pageStatus.classList.remove('hidden');
    pageStatus.classList.remove('border-red-200', 'bg-red-50', 'text-red-700');
    pageStatus.classList.remove('border-emerald-200', 'bg-emerald-50', 'text-emerald-700');

    if (tone === 'error') {
        pageStatus.classList.add('border-red-200', 'bg-red-50', 'text-red-700');
    } else if (tone === 'success') {
        pageStatus.classList.add('border-emerald-200', 'bg-emerald-50', 'text-emerald-700');
    }

    pageStatus.textContent = message;
}

function clearPageStatus() {
    pageStatus.classList.add('hidden');
    pageStatus.textContent = '';
}

async function fetchJson(path, options = {}) {
    const response = await fetch(`${API_URL}${path}`, options);
    if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `Request failed with status ${response.status}`);
    }
    if (response.status === 204) {
        return null;
    }
    return response.json();
}

async function loadData() {
    clearPageStatus();
    try {
        const eventsResponse = await fetchJson('/v1/events');
        events = eventsResponse;
        
        await loadCheckWindowsForEvents();
        renderEvents();
    } catch (error) {
        setPageStatus(`Failed to load events: ${error.message}`, 'error');
        eventsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="4">Unable to load events.</td>
            </tr>
        `;
    }
}

async function loadCheckWindowsForEvents() {
    eventCheckWindows = new Map();
    
    const results = await Promise.all(
        events.map(async event => {
            try {
                const windows = await fetchJson(`/v1/admin/events/${event.id}/check-windows`);
                return [event.id, windows];
            } catch (error) {
                return [event.id, []];
            }
        })
    );

    results.forEach(([eventId, windows]) => {
        eventCheckWindows.set(eventId, windows);
    });
}

function renderEvents() {
    if (!events.length) {
        eventsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="4">No events found.</td>
            </tr>
        `;
        return;
    }

    eventsBody.innerHTML = '';
    events.forEach(event => {
        const windows = eventCheckWindows.get(event.id) || [];
        const count = windows.length;
        
        const row = document.createElement('tr');
        row.innerHTML = `
            <td class="px-4 py-4 font-medium text-slate-900">${event.name || 'Unnamed event'}</td>
            <td class="px-4 py-4 text-slate-600">${event.planning_center_id || '-'}</td>
            <td class="px-4 py-4 text-slate-600">
                <span class="font-semibold">${count}</span> window${count !== 1 ? 's' : ''}
            </td>
            <td class="px-4 py-4">
                <a href="/admin/event-check-windows-detail?eventId=${event.id}" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-800 bg-slate-800 px-3 py-1.5 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-700">
                    Show all
                </a>
            </td>
        `;
        
        eventsBody.appendChild(row);
    });
}

document.addEventListener('DOMContentLoaded', () => {
    loadData();
});
