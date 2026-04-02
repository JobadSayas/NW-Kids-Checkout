const API_URL = '';

const pageStatus = document.getElementById('page-status');
const windowsBody = document.getElementById('windows-body');
const eventNameEl = document.getElementById('event-name');
const eventPcIdEl = document.getElementById('event-pc-id');
const windowModal = document.getElementById('window-modal');
const windowForm = document.getElementById('window-form');
const modalTitle = document.getElementById('modal-title');
const modalStatus = document.getElementById('modal-status');

const dayNames = ['', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

let eventId = null;
let event = null;
let windows = [];
let pendingRequests = 0;

function getEventIdFromUrl() {
    const params = new URLSearchParams(window.location.search);
    return params.get('eventId');
}

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
    if (!timeStr) return false;
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
    eventId = getEventIdFromUrl();
    
    if (!eventId) {
        setPageStatus('No event ID specified', 'error');
        windowsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="4">No event specified.</td>
            </tr>
        `;
        return;
    }

    clearPageStatus();
    try {
        const eventsResponse = await fetchJson('/v1/events');
        event = eventsResponse.find(e => e.id === parseInt(eventId));
        
        if (!event) {
            setPageStatus('Event not found', 'error');
            windowsBody.innerHTML = `
                <tr>
                    <td class="px-4 py-6 text-center text-slate-500" colspan="4">Event not found.</td>
                </tr>
            `;
            return;
        }

        eventNameEl.textContent = event.name || 'Unnamed event';
        eventPcIdEl.textContent = event.planning_center_id || '-';

        windows = await fetchJson(`/v1/admin/events/${eventId}/check-windows`);
        renderWindows();
    } catch (error) {
        setPageStatus(`Failed to load data: ${error.message}`, 'error');
        windowsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="4">Unable to load data.</td>
            </tr>
        `;
    }
}

function formatCheckWindow(window) {
    const startDay = dayNames[window.start_day_of_week] || window.start_day_of_week;
    const endDay = dayNames[window.end_day_of_week] || window.end_day_of_week;
    const startTime = window.start_time || '';
    const endTime = window.end_time || '';
    const tz = window.timezone || '';
    
    return `${startDay} ${startTime} - ${endDay} ${endTime} (${tz})`;
}

function renderWindows() {
    if (!windows.length) {
        windowsBody.innerHTML = `
            <tr>
                <td class="px-4 py-6 text-center text-slate-500" colspan="4">No check windows configured. Add one to get started.</td>
            </tr>
        `;
        return;
    }

    windowsBody.innerHTML = '';
    windows.forEach(w => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td class="px-4 py-4 text-slate-900">${dayNames[w.start_day_of_week]} ${w.start_time || ''}</td>
            <td class="px-4 py-4 text-slate-900">${dayNames[w.end_day_of_week]} ${w.end_time || ''}</td>
            <td class="px-4 py-4 text-slate-600">${w.timezone || '-'}</td>
            <td class="px-4 py-4">
                <div class="flex items-center gap-2">
                    <button onclick="openEditWindow(${w.id}, ${w.start_day_of_week}, '${w.start_time}', ${w.end_day_of_week}, '${w.end_time}', '${w.timezone}')" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-600 shadow-sm transition hover:border-slate-400 hover:text-slate-900">Edit</button>
                    <button onclick="deleteWindow(${w.id})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600 shadow-sm transition hover:border-red-400 hover:text-red-800">Delete</button>
                </div>
            </td>
        `;
        
        windowsBody.appendChild(row);
    });
}

function openAddWindow() {
    clearFieldErrors();
    clearModalError();
    document.getElementById('modal-title').textContent = 'Add Check Window';
    document.getElementById('window-id').value = '';
    document.getElementById('event-id').value = eventId;
    document.getElementById('start-day').value = '7';
    document.getElementById('start-time').value = '';
    document.getElementById('end-day').value = '7';
    document.getElementById('end-time').value = '';
    document.getElementById('timezone').value = 'America/Chicago';
    
    windowModal.classList.remove('hidden');
}

function openEditWindow(windowId, startDay, startTime, endDay, endTime, timezone) {
    clearFieldErrors();
    clearModalError();
    document.getElementById('modal-title').textContent = 'Edit Check Window';
    document.getElementById('window-id').value = windowId;
    document.getElementById('event-id').value = eventId;
    document.getElementById('start-day').value = String(startDay);
    document.getElementById('start-time').value = startTime;
    document.getElementById('end-day').value = String(endDay);
    document.getElementById('end-time').value = endTime;
    document.getElementById('timezone').value = timezone;
    
    windowModal.classList.remove('hidden');
}

function closeModal() {
    clearFieldErrors();
    clearModalError();
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
    const startDay = parseInt(document.getElementById('start-day').value);
    const startTime = document.getElementById('start-time').value.trim();
    const endDay = parseInt(document.getElementById('end-day').value);
    const endTime = document.getElementById('end-time').value.trim();
    const timezone = document.getElementById('timezone').value;
    
    const payload = {
        start_day_of_week: startDay,
        start_time: startTime,
        end_day_of_week: endDay,
        end_time: endTime,
        timezone: timezone
    };
    
    clearPageStatus();
    pendingRequests += 1;
    
    try {
        if (windowId) {
            await fetchJson(`/v1/admin/events/${eventId}/check-windows/${windowId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            setPageStatus('Check window updated successfully', 'success');
        } else {
            await fetchJson(`/v1/admin/events/${eventId}/check-windows`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            setPageStatus('Check window created successfully', 'success');
        }
        
        closeModal();
        windows = await fetchJson(`/v1/admin/events/${eventId}/check-windows`);
        renderWindows();
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
        await fetchJson(`/v1/admin/events/${eventId}/check-windows/${windowId}`, {
            method: 'DELETE'
        });
        setPageStatus('Check window deleted successfully', 'success');
        windows = await fetchJson(`/v1/admin/events/${eventId}/check-windows`);
        renderWindows();
    } catch (error) {
        setPageStatus(`Delete failed: ${error.message}`, 'error');
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

window.closeModal = closeModal;
window.openAddWindow = openAddWindow;
window.openEditWindow = openEditWindow;
window.deleteWindow = deleteWindow;
