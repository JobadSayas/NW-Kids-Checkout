const API_URL = '';

const pageStatus = document.getElementById('page-status');
const eventsBody = document.getElementById('events-body');
const prevButton = document.getElementById('prev-page');
const nextButton = document.getElementById('next-page');

const previousPages = [];
let currentPath = '/v1/admin/planningcenter/events';
let nextURL = '';
let isLoading = false;

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

function setLoadingState(loading) {
    isLoading = loading;
    prevButton.disabled = loading || previousPages.length === 0;
    nextButton.disabled = loading || !nextURL;
}

async function loadEvents(path = '/v1/admin/planningcenter/events') {
    clearPageStatus();
    setLoadingState(true);
    try {
        currentPath = path;
        const response = await fetchJson(path);
        renderEvents((response && response.events) || []);
        nextURL = (response && response.links && response.links.next) || '';
    } catch (error) {
        setPageStatus(`Failed to load events: ${error.message}`, 'error');
        eventsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="3">Unable to load events.</td>
            </tr>
        `;
        nextURL = '';
    } finally {
        setLoadingState(false);
    }
}

function renderEvents(events) {
    if (!events.length) {
        eventsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="3">No events found.</td>
            </tr>
        `;
        return;
    }

    const sorted = [...events].sort((a, b) => a.name.localeCompare(b.name));
    eventsBody.innerHTML = '';
    sorted.forEach(event => {
        const row = document.createElement('tr');
        const eventName = event.name || 'Unnamed event';
        const eventNameParam = encodeURIComponent(eventName);
        row.innerHTML = `
            <td class="px-4 py-4 font-medium text-slate-900">${eventName}</td>
            <td class="px-4 py-4 text-slate-600">${event.id}</td>
            <td class="px-4 py-4">
                <a class="inline-flex items-center gap-2 text-sm font-semibold text-emerald-600 hover:text-emerald-800" href="/admin/planningcenter/events/${event.id}?name=${eventNameParam}">
                    View locations
                    <span aria-hidden="true">→</span>
                </a>
            </td>
        `;
        eventsBody.appendChild(row);
    });
}

document.addEventListener('DOMContentLoaded', () => {
    prevButton.addEventListener('click', () => {
        if (!previousPages.length || isLoading) {
            return;
        }
        const previous = previousPages.pop();
        loadEvents(previous);
    });

    nextButton.addEventListener('click', () => {
        if (!nextURL || isLoading) {
            return;
        }
        previousPages.push(currentPath);
        loadEvents(nextURL);
    });

    loadEvents();
});
