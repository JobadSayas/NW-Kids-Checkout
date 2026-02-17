const API_URL = '';

const pageStatus = document.getElementById('page-status');
const locationsBody = document.getElementById('locations-body');
const eventIdLabel = document.getElementById('event-id-label');
const eventNameLabel = document.getElementById('event-name-label');
const addEventButton = document.getElementById('add-event-button');
const eventAddedStatus = document.getElementById('event-added-status');

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

async function createEvent(eventId) {
    const response = await fetch(`${API_URL}/admin/api/events`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            planning_center_id: eventId
        })
    });

    if (!response.ok) {
        const message = await response.text();
        const error = new Error(message || `Request failed with status ${response.status}`);
        error.status = response.status;
        throw error;
    }

    if (response.status === 204) {
        return null;
    }

    return response.json();
}

async function getExistingEvent(eventId) {
    const response = await fetch(`${API_URL}/admin/api/events/lookup?planning_center_id=${encodeURIComponent(eventId)}`);
    if (response.status === 404) {
        return null;
    }
    if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `Request failed with status ${response.status}`);
    }
    return response.json();
}

function getEventIdFromPath() {
    const parts = window.location.pathname.split('/').filter(Boolean);
    return parts.length ? parts[parts.length - 1] : '';
}

function getEventNameFromQuery() {
    const params = new URLSearchParams(window.location.search);
    const name = params.get('name');
    return name ? decodeURIComponent(name) : '';
}

function getParentName(locationsById, parentId) {
    if (!parentId) {
        return '-';
    }
    const parent = locationsById.get(parentId);
    return parent ? parent.name : 'Unknown parent';
}

async function loadLocations(eventId) {
    clearPageStatus();
    try {
        const locations = await fetchJson(`/admin/api/planningcenter/events/${eventId}/locations`);
        renderLocations(locations || []);
    } catch (error) {
        setPageStatus(`Failed to load locations: ${error.message}`, 'error');
        locationsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="3">Unable to load locations.</td>
            </tr>
        `;
    }
}

function renderLocations(locations) {
    if (!locations.length) {
        locationsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="3">No locations found.</td>
            </tr>
        `;
        return;
    }

    const locationsById = new Map();
    locations.forEach(location => {
        locationsById.set(location.id, location);
    });

    locationsBody.innerHTML = '';
    locations.forEach(location => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td class="px-4 py-4 font-medium text-slate-900">${location.name || 'Unnamed location'}</td>
            <td class="px-4 py-4 text-slate-600">${getParentName(locationsById, location.parent_id)}</td>
            <td class="px-4 py-4 text-slate-600">${location.id}</td>
        `;
        locationsBody.appendChild(row);
    });
}

document.addEventListener('DOMContentLoaded', () => {
    const eventId = getEventIdFromPath();
    eventIdLabel.textContent = eventId || '-';
    const eventName = getEventNameFromQuery();
    eventNameLabel.textContent = eventName || '-';

    if (!eventId) {
        setPageStatus('Missing event id in URL.', 'error');
        locationsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="3">No event selected.</td>
            </tr>
        `;
        if (addEventButton) {
            addEventButton.disabled = true;
        }
        return;
    }

    if (addEventButton) {
        addEventButton.addEventListener('click', async () => {
            addEventButton.disabled = true;
            const originalLabel = addEventButton.textContent;
            addEventButton.textContent = 'Adding event...';
            clearPageStatus();
            try {
                await createEvent(eventId);
                window.location.assign('/admin/locations');
            } catch (error) {
                if (error.status === 409) {
                    setPageStatus('Event already exists in the system.', 'error');
                } else {
                    setPageStatus(`Failed to add event: ${error.message}`, 'error');
                }
                addEventButton.disabled = false;
                addEventButton.textContent = originalLabel;
            }
        });
    }

    if (addEventButton) {
        getExistingEvent(eventId)
            .then(event => {
                console.log('[planningcenter-event] lookup result', { eventId, exists: Boolean(event), event });
                if (event) {
                    addEventButton.classList.add('hidden');
                    addEventButton.classList.remove('inline-flex');
                    if (eventAddedStatus) {
                        eventAddedStatus.classList.remove('hidden');
                    }
                }
            })
            .catch(error => {
                console.error('[planningcenter-event] lookup failed', error);
                setPageStatus(`Failed to check event status: ${error.message}`, 'error');
            });
    }

    loadLocations(eventId);
});
