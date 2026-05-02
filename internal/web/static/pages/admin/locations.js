const API_URL = '';

const pageStatus = document.getElementById('page-status');
const locationsBody = document.getElementById('locations-body');

let locationGroups = [];
let locations = [];
let eventsById = new Map();
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
        const respLocations = await fetch('/v1/locations');
        const locationsResponse = await respLocations.json();
        const [groups, eventsResponse] = await Promise.all([
            fetchJson('/v1/location_groups'),
            fetchJson('/v1/events')
        ]);

        locationGroups = groups;
        locations = locationsResponse;
        eventsById = new Map(eventsResponse.map(event => [Number(event.id), event]));

        renderLocations();
    } catch (error) {
        setPageStatus(`Failed to load locations: ${error.message}`, 'error');
        locationsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="5">Unable to load locations.</td>
            </tr>
        `;
    }
}

function getLocationGroupName(groupId) {
    if (!groupId) {
        return 'Unassigned';
    }
    const group = locationGroups.find(item => item.id === groupId);
    return group ? group.name : 'Unknown group';
}

function getPlanningCenterParentName(location) {
    if (!location.planning_center_parent_id) {
        return '-';
    }
    const parentId = String(location.planning_center_parent_id);
    const parent = locations.find(item => String(item.planning_center_id) === parentId);
    return parent ? parent.name : 'Unknown parent';
}

function getEventName(location) {
    if (!location.event_id) {
        return 'Unassigned';
    }
    const event = eventsById.get(location.event_id);
    if (!event) {
        return 'Unknown event';
    }
    return event.name || 'Unnamed event';
}

function renderLocations() {
    locationsBody.innerHTML = '';

    const allEventIds = Array.from(eventsById.keys()).sort((a, b) => {
        const eventA = eventsById.get(a);
        const eventB = eventsById.get(b);
        return (eventA?.name || '').localeCompare(eventB?.name || '');
    });

    if (allEventIds.length === 0) {
        locationsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="5">No events found.</td>
            </tr>
        `;
        return;
    }

    allEventIds.forEach(eventId => {
        const event = eventsById.get(eventId);
        const eventName = event?.name || 'Unnamed event';

        const eventLocations = locations.filter(loc => Number(loc.event_id) === eventId);

        const eventRow = document.createElement('tr');
        eventRow.classList.add('bg-slate-100');
        eventRow.innerHTML = `
            <td class="px-4 py-3 font-semibold text-slate-800">${eventName}</td>
            <td class="px-4 py-3 text-slate-600">-</td>
            <td class="px-4 py-3 text-slate-600">-</td>
            <td class="px-4 py-3">
                <select class="event-group-select w-full rounded-md border border-slate-300 bg-white px-2 py-1 text-sm">
                    <option value="">Unassigned</option>
                    ${locationGroups.map(group => `
                        <option value="${group.id}" ${event.location_group_id === group.id ? 'selected' : ''}>${group.name}</option>
                    `).join('')}
                </select>
            </td>
            <td class="px-4 py-3">
                <label class="flex items-center gap-3 text-sm text-slate-700">
                    <span class="relative inline-flex h-5 w-9 items-center">
                        <input type="checkbox" class="event-auto-fetch-toggle peer sr-only" ${event.auto_fetch ? 'checked' : ''}>
                        <span class="h-5 w-9 rounded-full bg-slate-200 transition peer-checked:bg-emerald-500"></span>
                        <span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition peer-checked:translate-x-4"></span>
                    </span>
                </label>
            </td>
        `;

        const eventSelect = eventRow.querySelector('.event-group-select');
        const eventToggle = eventRow.querySelector('.event-auto-fetch-toggle');

        eventSelect.addEventListener('change', async () => {
            const previousValue = event.location_group_id ?? '';
            const nextValue = eventSelect.value === '' ? null : Number(eventSelect.value);

            eventSelect.disabled = true;

            const success = await updateEvent(event, { location_group_id: nextValue });
            if (!success) {
                eventSelect.value = previousValue === null ? '' : String(previousValue);
            } else {
                event.location_group_id = nextValue;
            }

            eventSelect.disabled = false;
        });

        eventToggle.addEventListener('change', async () => {
            const previousValue = event.auto_fetch;
            const nextValue = eventToggle.checked;

            eventToggle.disabled = true;

            const success = await updateEvent(event, { auto_fetch: nextValue });
            if (!success) {
                eventToggle.checked = previousValue;
            } else {
                event.auto_fetch = nextValue;
            }

            eventToggle.disabled = false;
        });

        locationsBody.appendChild(eventRow);

        if (eventLocations.length === 0) {
            const emptyRow = document.createElement('tr');
            emptyRow.innerHTML = `
                <td class="px-4 py-4 text-slate-500 italic" colspan="5">No locations</td>
            `;
            locationsBody.appendChild(emptyRow);
            return;
        }

        const sortedLocations = NWKidsLocationSort.sortLocationsByPlanningCenterHierarchy(eventLocations);

        sortedLocations.forEach(location => {
            const row = document.createElement('tr');
            row.dataset.locationId = location.id;

            row.innerHTML = `
                <td class="px-4 py-4 font-medium text-slate-900">${location.name}</td>
                <td class="px-4 py-4 text-slate-600">${getPlanningCenterParentName(location)}</td>
                <td class="px-4 py-4 text-slate-600">${getEventName(location)}</td>
                <td class="px-4 py-4">
                    <select class="location-group-select w-full rounded-md border border-slate-300 bg-white px-2 py-1 text-sm">
                        <option value="">Unassigned</option>
                        ${locationGroups.map(group => `
                            <option value="${group.id}" ${location.location_group_id === group.id ? 'selected' : ''}>${group.name}</option>
                        `).join('')}
                    </select>
                </td>
                <td class="px-4 py-4">
                    <label class="flex items-center gap-3 text-sm text-slate-700">
                        <span class="relative inline-flex h-5 w-9 items-center">
                            <input type="checkbox" class="auto-fetch-toggle peer sr-only" ${location.auto_fetch ? 'checked' : ''}>
                            <span class="h-5 w-9 rounded-full bg-slate-200 transition peer-checked:bg-emerald-500"></span>
                            <span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition peer-checked:translate-x-4"></span>
                        </span>
                    </label>
                </td>
            `;

            const select = row.querySelector('.location-group-select');
            const toggle = row.querySelector('.auto-fetch-toggle');

            select.addEventListener('change', async () => {
                const previousValue = location.location_group_id ?? '';
                const nextValue = select.value === '' ? null : Number(select.value);

                select.disabled = true;

                const success = await updateLocation(location, { location_group_id: nextValue });
                if (!success) {
                    select.value = previousValue === null ? '' : String(previousValue);
                } else {
                    location.location_group_id = nextValue;
                }

                select.disabled = false;
            });

            toggle.addEventListener('change', async () => {
                const previousValue = location.auto_fetch;
                const nextValue = toggle.checked;

                toggle.disabled = true;

                const success = await updateLocation(location, { auto_fetch: nextValue });
                if (!success) {
                    toggle.checked = previousValue;
                } else {
                    location.auto_fetch = nextValue;
                }

                toggle.disabled = false;
            });

            locationsBody.appendChild(row);
        });
    });
}

async function updateLocation(location, payload) {
    clearPageStatus();
    pendingRequests += 1;
    try {
        const updated = await fetchJson(`/v1/locations/${location.id}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (updated) {
            location.auto_fetch = updated.auto_fetch;
            location.location_group_id = updated.location_group_id;
        }
        return true;
    } catch (error) {
        setPageStatus(`Update failed for ${location.name}: ${error.message}`, 'error');
        return false;
    } finally {
        pendingRequests = Math.max(0, pendingRequests - 1);
    }
}

async function updateEvent(event, payload) {
    clearPageStatus();
    pendingRequests += 1;
    try {
        const updated = await fetchJson(`/v1/admin/events/${event.id}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (updated) {
            event.auto_fetch = updated.auto_fetch;
            event.location_group_id = updated.location_group_id;
        }
        return true;
    } catch (error) {
        setPageStatus(`Update failed for ${event.name}: ${error.message}`, 'error');
        return false;
    } finally {
        pendingRequests = Math.max(0, pendingRequests - 1);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    window.addEventListener('beforeunload', event => {
        if (pendingRequests > 0) {
            event.preventDefault();
            event.returnValue = '';
        }
    });

    loadData();
});
