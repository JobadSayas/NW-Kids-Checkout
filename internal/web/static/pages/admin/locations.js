const API_URL = '';

const pageStatus = document.getElementById('page-status');
const locationsBody = document.getElementById('locations-body');
const windowModal = document.getElementById('window-modal');
const windowForm = document.getElementById('window-form');
const modalTitle = document.getElementById('modal-title');
const modalStatus = document.getElementById('modal-status');
const modalWindowsList = document.getElementById('modal-windows-list');
const addWindowButton = document.getElementById('add-window-button');

const dayNames = ['', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

let locationGroups = [];
let locations = [];
let eventsById = new Map();
let pendingRequests = 0;
let activeEvent = null;
let windows = [];

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
                <td class="px-4 py-6 text-center text-slate-500" colspan="6">Unable to load locations.</td>
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
                <td class="px-4 py-6 text-center text-slate-500" colspan="6">No events found.</td>
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
                <label class="inline-flex cursor-pointer items-center gap-3 text-sm text-slate-700">
                    <span class="relative inline-flex h-5 w-9 items-center">
                        <input type="checkbox" class="event-auto-fetch-toggle peer absolute opacity-0 w-0 h-0" ${event.auto_fetch ? 'checked' : ''}>
                        <span class="toggle-bg h-5 w-9 rounded-full" style="background-color: ${event.auto_fetch ? 'var(--color-emerald-500)' : 'var(--color-slate-200)'}"></span>
                        <span class="toggle-knob absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow" style="transform: ${event.auto_fetch ? 'translateX(1rem)' : 'translateX(0)'}"></span>
                    </span>
                </label>
            </td>
            <td class="px-4 py-3">
                <button onclick="openCheckWindowsModal(${eventId})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-800 bg-slate-800 px-3 py-1.5 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-700">
                    Check Windows
                </button>
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

            const toggleBg = eventRow.querySelector('.toggle-bg');
            const toggleKnob = eventRow.querySelector('.toggle-knob');

            if (nextValue) {
                toggleBg.style.backgroundColor = 'var(--color-emerald-500)';
                toggleKnob.style.transform = 'translateX(1rem)';
            } else {
                toggleBg.style.backgroundColor = 'var(--color-slate-200)';
                toggleKnob.style.transform = 'translateX(0)';
            }

            eventToggle.disabled = true;

            const success = await updateEvent(event, { auto_fetch: nextValue });
            if (!success) {
                eventToggle.checked = previousValue;
                if (previousValue) {
                    toggleBg.style.backgroundColor = 'var(--color-emerald-500)';
                    toggleKnob.style.transform = 'translateX(1rem)';
                } else {
                    toggleBg.style.backgroundColor = 'var(--color-slate-200)';
                    toggleKnob.style.transform = 'translateX(0)';
                }
            } else {
                event.auto_fetch = nextValue;
                renderLocations();
                eventToggle.disabled = false;
                return;
            }

            eventToggle.disabled = false;
        });

        locationsBody.appendChild(eventRow);

        if (eventLocations.length === 0) {
            const emptyRow = document.createElement('tr');
            emptyRow.innerHTML = `
                <td class="px-4 py-4 text-slate-500 italic" colspan="6">No locations</td>
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
                <td class="px-4 py-4"></td>
                <td class="px-4 py-4"></td>
            `;

            const select = row.querySelector('.location-group-select');

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

function setModalError(message) {
    modalStatus.classList.remove('hidden');
    modalStatus.textContent = message;
}

function clearModalError() {
    modalStatus.classList.add('hidden');
    modalStatus.textContent = '';
}

function showFieldError(fieldId, message) {
    const errorEl = document.getElementById(`${fieldId}-error`);
    const inputEl = document.getElementById(fieldId);
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.classList.remove('hidden');
    }
    if (inputEl) {
        inputEl.classList.add('border-red-300');
    }
}

function clearFieldErrors() {
    ['start-time', 'end-time'].forEach(fieldId => {
        const errorEl = document.getElementById(`${fieldId}-error`);
        const inputEl = document.getElementById(fieldId);
        if (errorEl) {
            errorEl.classList.add('hidden');
            errorEl.textContent = '';
        }
        if (inputEl) {
            inputEl.classList.remove('border-red-300');
        }
    });
}

function isValidTime(timeStr) {
    if (!timeStr) {
        return false;
    }
    const regex = /^([01]?[0-9]|2[0-3]):[0-5][0-9]$/;
    return regex.test(timeStr);
}

function validateForm() {
    let isValid = true;
    clearFieldErrors();
    clearModalError();

    const startTime = document.getElementById('start-time').value.trim();
    const endTime = document.getElementById('end-time').value.trim();

    if (!startTime) {
        showFieldError('start-time', 'Start time is required');
        isValid = false;
    } else if (!isValidTime(startTime)) {
        showFieldError('start-time', 'Invalid time format. Use HH:MM (24-hour)');
        isValid = false;
    }

    if (!endTime) {
        showFieldError('end-time', 'End time is required');
        isValid = false;
    } else if (!isValidTime(endTime)) {
        showFieldError('end-time', 'Invalid time format. Use HH:MM (24-hour)');
        isValid = false;
    }

    return isValid;
}

function formatCheckWindow(window) {
    const startDay = dayNames[window.start_day_of_week] || window.start_day_of_week;
    const endDay = dayNames[window.end_day_of_week] || window.end_day_of_week;
    const startTime = window.start_time || '';
    const endTime = window.end_time || '';
    const tz = window.timezone || '';

    return `${startDay} ${startTime} - ${endDay} ${endTime} (${tz})`;
}

function showForm() {
    modalWindowsList.classList.add('hidden');
    addWindowButton.style.display = 'none';
    windowForm.classList.remove('hidden');
}

function cancelForm() {
    clearFieldErrors();
    clearModalError();
    windowForm.classList.add('hidden');
    modalWindowsList.classList.remove('hidden');
    addWindowButton.style.display = '';
    modalTitle.textContent = `${activeEvent?.name || 'Event'} - Check Windows`;
}

function renderModalWindows() {
    if (!windows.length) {
        modalWindowsList.innerHTML = `
            <p class="text-sm text-slate-500">No check windows configured. Add one to get started.</p>
        `;
        return;
    }

    modalWindowsList.innerHTML = windows.map(w => `
        <div class="flex items-center justify-between gap-4 border-b border-slate-100 py-2 last:border-b-0">
            <span class="text-sm text-slate-800">${formatCheckWindow(w)}</span>
            <span class="flex shrink-0 items-center gap-2">
                <button onclick="openEditWindow(${w.id})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-600 shadow-sm transition hover:border-slate-400 hover:text-slate-900">Edit</button>
                <button onclick="deleteWindow(${w.id})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600 shadow-sm transition hover:border-red-400 hover:text-red-800">Delete</button>
            </span>
        </div>
    `).join('');
}

async function openCheckWindowsModal(eventId) {
    activeEvent = eventsById.get(eventId) || null;
    clearFieldErrors();
    clearModalError();
    cancelForm();
    windowModal.classList.remove('hidden');
    modalWindowsList.innerHTML = `<p class="text-sm text-slate-500">Loading check windows...</p>`;

    try {
        windows = await fetchJson(`/v1/admin/events/${eventId}/check-windows`);
        renderModalWindows();
    } catch (error) {
        modalWindowsList.innerHTML = `<p class="text-sm text-red-600">Failed to load check windows: ${error.message}</p>`;
    }
}

function openAddWindow() {
    clearFieldErrors();
    clearModalError();
    document.getElementById('window-id').value = '';
    document.getElementById('start-day').value = '7';
    document.getElementById('start-time').value = '';
    document.getElementById('end-day').value = '7';
    document.getElementById('end-time').value = '';
    document.getElementById('timezone').value = 'America/Chicago';

    modalTitle.textContent = `${activeEvent?.name || 'Event'} - Add Check Window`;
    showForm();
}

function openEditWindow(windowId) {
    const window = windows.find(item => item.id === windowId);
    if (!window) {
        return;
    }

    clearFieldErrors();
    clearModalError();
    document.getElementById('window-id').value = windowId;
    document.getElementById('start-day').value = String(window.start_day_of_week);
    document.getElementById('start-time').value = window.start_time;
    document.getElementById('end-day').value = String(window.end_day_of_week);
    document.getElementById('end-time').value = window.end_time;
    document.getElementById('timezone').value = window.timezone;

    modalTitle.textContent = `${activeEvent?.name || 'Event'} - Edit Check Window`;
    showForm();
}

function closeModal() {
    clearFieldErrors();
    clearModalError();
    cancelForm();
    windowModal.classList.add('hidden');
}

function handleKeydown(event) {
    if (event.key === 'Escape' && !windowModal.classList.contains('hidden')) {
        closeModal();
    }
}

async function handleFormSubmit(event) {
    event.preventDefault();

    if (!validateForm()) {
        return;
    }

    const windowId = document.getElementById('window-id').value;
    const payload = {
        start_day_of_week: parseInt(document.getElementById('start-day').value, 10),
        start_time: document.getElementById('start-time').value.trim(),
        end_day_of_week: parseInt(document.getElementById('end-day').value, 10),
        end_time: document.getElementById('end-time').value.trim(),
        timezone: document.getElementById('timezone').value
    };

    clearPageStatus();
    pendingRequests += 1;

    try {
        if (windowId) {
            await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows/${windowId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            setPageStatus('Check window updated successfully', 'success');
        } else {
            await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            setPageStatus('Check window created successfully', 'success');
        }

        closeModal();
    } catch (error) {
        setModalError(error.message);
    } finally {
        pendingRequests = Math.max(0, pendingRequests - 1);
    }
}

async function deleteWindow(windowId) {
    if (!confirm('Are you sure you want to delete this check window?')) {
        return;
    }

    clearPageStatus();
    pendingRequests += 1;

    try {
        await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows/${windowId}`, {
            method: 'DELETE'
        });
        closeModal();
        setPageStatus('Check window deleted successfully', 'success');
    } catch (error) {
        setModalError(error.message);
    } finally {
        pendingRequests = Math.max(0, pendingRequests - 1);
    }
}

windowForm.addEventListener('submit', handleFormSubmit);

document.addEventListener('DOMContentLoaded', () => {
    document.addEventListener('keydown', handleKeydown);
    window.addEventListener('beforeunload', event => {
        if (pendingRequests > 0) {
            event.preventDefault();
            event.returnValue = '';
        }
    });

    loadData();
});

window.openCheckWindowsModal = openCheckWindowsModal;
window.openAddWindow = openAddWindow;
window.openEditWindow = openEditWindow;
window.closeModal = closeModal;
window.cancelForm = cancelForm;
window.deleteWindow = deleteWindow;
