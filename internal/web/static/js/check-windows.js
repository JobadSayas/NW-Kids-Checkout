const dayNames = ['', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

const modalMarkup = `
<div id="window-modal" class="fixed inset-0 z-50 hidden">
    <div class="fixed inset-0 bg-black/50" onclick="closeModal()"></div>
    <div class="fixed left-1/2 top-1/2 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl bg-white p-6 shadow-xl">
        <h2 id="modal-title" class="mb-4 text-xl font-semibold">Check Windows</h2>
        <div id="modal-status" class="hidden mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"></div>

        <div id="modal-windows-list" class="mb-4"></div>

        <button id="add-window-button" onclick="openAddWindow()" class="mb-4 inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-800 bg-slate-800 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-700">
            Add Window
        </button>

        <form id="window-form" class="hidden">
            <input type="hidden" id="window-id">

            <div class="mb-4">
                <label for="start-day" class="block text-sm font-medium text-slate-700">Start Day</label>
                <select id="start-day" class="mt-1 w-full rounded-md border border-slate-300 px-3 py-2">
                    <option value="1">Monday</option>
                    <option value="2">Tuesday</option>
                    <option value="3">Wednesday</option>
                    <option value="4">Thursday</option>
                    <option value="5">Friday</option>
                    <option value="6">Saturday</option>
                    <option value="7" selected>Sunday</option>
                </select>
            </div>

            <div class="mb-4">
                <label for="start-time" class="block text-sm font-medium text-slate-700">Start Time</label>
                <div class="mt-1 flex gap-2">
                    <input type="text" id="start-time" placeholder="9:00" autocomplete="off" class="w-full rounded-md border border-slate-300 px-3 py-2">
                    <select id="start-time-ampm" class="w-28 rounded-md border border-slate-300 px-3 py-2">
                        <option value="AM" selected>AM</option>
                        <option value="PM">PM</option>
                    </select>
                </div>
                <p id="start-time-error" class="mt-1 text-xs text-red-600 hidden"></p>
            </div>

            <div class="mb-4">
                <label for="end-day" class="block text-sm font-medium text-slate-700">End Day</label>
                <select id="end-day" class="mt-1 w-full rounded-md border border-slate-300 px-3 py-2">
                    <option value="1">Monday</option>
                    <option value="2">Tuesday</option>
                    <option value="3">Wednesday</option>
                    <option value="4">Thursday</option>
                    <option value="5">Friday</option>
                    <option value="6">Saturday</option>
                    <option value="7" selected>Sunday</option>
                </select>
            </div>

            <div class="mb-4">
                <label for="end-time" class="block text-sm font-medium text-slate-700">End Time</label>
                <div class="mt-1 flex gap-2">
                    <input type="text" id="end-time" placeholder="12:00" autocomplete="off" class="w-full rounded-md border border-slate-300 px-3 py-2">
                    <select id="end-time-ampm" class="w-28 rounded-md border border-slate-300 px-3 py-2">
                        <option value="AM" selected>AM</option>
                        <option value="PM">PM</option>
                    </select>
                </div>
                <p id="end-time-error" class="mt-1 text-xs text-red-600 hidden"></p>
            </div>

            <div class="mb-6">
                <label for="timezone" class="block text-sm font-medium text-slate-700">Timezone</label>
                <select id="timezone" class="mt-1 w-full rounded-md border border-slate-300 px-3 py-2">
                    <option value="America/Chicago">America/Chicago (CT)</option>
                    <option value="America/Los_Angeles">America/Los_Angeles (PT)</option>
                    <option value="America/Denver">America/Denver (MT)</option>
                    <option value="America/New_York">America/New_York (ET)</option>
                    <option value="UTC">UTC</option>
                </select>
            </div>

            <div class="flex justify-end gap-3">
                <button type="button" onclick="cancelForm()" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-600 shadow-sm transition hover:border-slate-400 hover:text-slate-900">Cancel</button>
                <button type="submit" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-800 bg-slate-800 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-700">Save</button>
            </div>
        </form>
    </div>
</div>`;

function ensureModalMarkup() {
    if (document.getElementById('window-modal')) {
        return;
    }
    const container = document.createElement('div');
    container.innerHTML = modalMarkup.trim();
    document.body.appendChild(container.firstChild);
}

const windowModal = () => document.getElementById('window-modal');
const windowForm = () => document.getElementById('window-form');
const modalTitle = () => document.getElementById('modal-title');
const modalStatus = () => document.getElementById('modal-status');
const modalWindowsList = () => document.getElementById('modal-windows-list');
const addWindowButton = () => document.getElementById('add-window-button');

let activeEvent = null;
let windows = [];
let modalPendingRequests = 0;

async function fetchJson(path, options = {}) {
    const response = await fetch(path, options);
    if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `Request failed with status ${response.status}`);
    }
    if (response.status === 204) {
        return null;
    }
    return response.json();
}

function setModalError(message) {
    modalStatus().classList.remove('hidden');
    modalStatus().textContent = message;
}

function clearModalError() {
    modalStatus().classList.add('hidden');
    modalStatus().textContent = '';
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
    const regex = /^\d{1,2}:\d{2}$/;
    if (!regex.test(timeStr)) {
        return false;
    }
    const [hourStr, minuteStr] = timeStr.split(':');
    const hour = parseInt(hourStr, 10);
    const minute = parseInt(minuteStr, 10);
    return hour >= 1 && hour <= 12 && minute >= 0 && minute <= 59;
}

function format24To12(time24) {
    const regex = /^([01]?[0-9]|2[0-3]):[0-5][0-9]$/;
    const match = regex.exec(time24);
    if (!match) {
        return null;
    }
    const [hourStr, minuteStr] = time24.split(':');
    let hour = parseInt(hourStr, 10);
    const ampm = hour < 12 ? 'AM' : 'PM';
    hour = hour % 12 || 12;
    return { hhmm: `${hour}:${minuteStr}`, ampm };
}

function to24Hour(timeStr, ampm) {
    const [hourStr, minuteStr] = timeStr.split(':');
    let hour = parseInt(hourStr, 10);
    if (ampm === 'AM') {
        hour = hour % 12;
    } else {
        hour = hour % 12 + 12;
    }
    return `${String(hour).padStart(2, '0')}:${minuteStr}`;
}

function validateForm() {
    let isValid = true;
    clearFieldErrors();
    clearModalError();

    const startTime = document.getElementById('start-time').value.trim();
    const startAmpm = document.getElementById('start-time-ampm').value;
    const endTime = document.getElementById('end-time').value.trim();
    const endAmpm = document.getElementById('end-time-ampm').value;

    if (!startTime) {
        showFieldError('start-time', 'Start time is required');
        isValid = false;
    } else if (!isValidTime(startTime) || !startAmpm) {
        showFieldError('start-time', 'Invalid time format. Use H:MM (e.g., 9:00) with AM/PM');
        isValid = false;
    }

    if (!endTime) {
        showFieldError('end-time', 'End time is required');
        isValid = false;
    } else if (!isValidTime(endTime) || !endAmpm) {
        showFieldError('end-time', 'Invalid time format. Use H:MM (e.g., 9:00) with AM/PM');
        isValid = false;
    }

    return isValid;
}

function formatCheckWindow(window) {
    const startDay = dayNames[window.start_day_of_week] || window.start_day_of_week;
    const endDay = dayNames[window.end_day_of_week] || window.end_day_of_week;
    const tz = window.timezone || '';

    const startTime = format24To12(window.start_time);
    const endTime = format24To12(window.end_time);

    const startDisplay = startTime ? `${startTime.hhmm} ${startTime.ampm}` : window.start_time || '';
    const endDisplay = endTime ? `${endTime.hhmm} ${endTime.ampm}` : window.end_time || '';

    return `${startDay} ${startDisplay} - ${endDay} ${endDisplay} (${tz})`;
}

function showForm() {
    modalWindowsList().classList.add('hidden');
    addWindowButton().style.display = 'none';
    windowForm().classList.remove('hidden');
}

function cancelForm() {
    clearFieldErrors();
    clearModalError();
    windowForm().classList.add('hidden');
    modalWindowsList().classList.remove('hidden');
    addWindowButton().style.display = '';
    modalTitle().textContent = `${activeEvent?.name || 'Event'} - Check Windows`;
}

function renderModalWindows() {
    if (!windows.length) {
        addWindowButton().style.display = 'none';
        modalWindowsList().innerHTML = `
            <p class="text-sm text-slate-500">No check windows configured.</p>
        `;
        return;
    }

    addWindowButton().style.display = '';
    modalWindowsList().innerHTML = windows.map(w => `
        <div class="flex items-center justify-between gap-4 border-b border-slate-100 py-2 last:border-b-0">
            <span class="text-sm text-slate-800">${formatCheckWindow(w)}</span>
            <span class="flex shrink-0 items-center gap-2">
                <button onclick="openEditWindow(${w.id})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-600 shadow-sm transition hover:border-slate-400 hover:text-slate-900">Edit</button>
                <button onclick="deleteWindow(${w.id})" class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-200 bg-white px-3 py-1.5 text-sm font-semibold text-red-600 shadow-sm transition hover:border-red-400 hover:text-red-800">Delete</button>
            </span>
        </div>
    `).join('');
}

async function openCheckWindowsModal(eventId, eventName) {
    activeEvent = { id: eventId, name: eventName || 'Event' };
    clearFieldErrors();
    clearModalError();
    cancelForm();
    windowModal().classList.remove('hidden');
    modalWindowsList().innerHTML = `<p class="text-sm text-slate-500">Loading check windows...</p>`;

    try {
        windows = await fetchJson(`/v1/admin/events/${eventId}/check-windows`);
        renderModalWindows();
        if (windows.length === 0) {
            openAddWindow();
        }
    } catch (error) {
        modalWindowsList().innerHTML = `<p class="text-sm text-red-600">Failed to load check windows: ${error.message}</p>`;
    }
}

function openAddWindow() {
    clearFieldErrors();
    clearModalError();
    document.getElementById('window-id').value = '';
    document.getElementById('start-day').value = '7';
    document.getElementById('start-time').value = '';
    document.getElementById('start-time-ampm').value = 'AM';
    document.getElementById('end-day').value = '7';
    document.getElementById('end-time').value = '';
    document.getElementById('end-time-ampm').value = 'AM';
    document.getElementById('timezone').value = 'America/Chicago';

    modalTitle().textContent = `${activeEvent?.name || 'Event'} - Add Check Window`;
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
    const startTime = format24To12(window.start_time);
    document.getElementById('start-time').value = startTime ? startTime.hhmm : window.start_time;
    document.getElementById('start-time-ampm').value = startTime ? startTime.ampm : 'AM';
    document.getElementById('end-day').value = String(window.end_day_of_week);
    const endTime = format24To12(window.end_time);
    document.getElementById('end-time').value = endTime ? endTime.hhmm : window.end_time;
    document.getElementById('end-time-ampm').value = endTime ? endTime.ampm : 'AM';
    document.getElementById('timezone').value = window.timezone;

    modalTitle().textContent = `${activeEvent?.name || 'Event'} - Edit Check Window`;
    showForm();
}

function closeModal() {
    clearFieldErrors();
    clearModalError();
    cancelForm();
    windowModal().classList.add('hidden');
}

function handleKeydown(event) {
    if (event.key === 'Escape' && !windowModal().classList.contains('hidden')) {
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
        start_time: to24Hour(document.getElementById('start-time').value.trim(), document.getElementById('start-time-ampm').value),
        end_day_of_week: parseInt(document.getElementById('end-day').value, 10),
        end_time: to24Hour(document.getElementById('end-time').value.trim(), document.getElementById('end-time-ampm').value),
        timezone: document.getElementById('timezone').value
    };

    window.clearPageStatus?.();
    modalPendingRequests += 1;

    try {
        if (windowId) {
            await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows/${windowId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            window.setPageStatus?.('Check window updated successfully', 'success');
        } else {
            await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            window.setPageStatus?.('Check window created successfully', 'success');
        }

        closeModal();
    } catch (error) {
        setModalError(error.message);
    } finally {
        modalPendingRequests = Math.max(0, modalPendingRequests - 1);
    }
}

async function deleteWindow(windowId) {
    if (!confirm('Are you sure you want to delete this check window?')) {
        return;
    }

    window.clearPageStatus?.();
    modalPendingRequests += 1;

    try {
        await fetchJson(`/v1/admin/events/${activeEvent.id}/check-windows/${windowId}`, {
            method: 'DELETE'
        });
        closeModal();
        window.setPageStatus?.('Check window deleted successfully', 'success');
    } catch (error) {
        setModalError(error.message);
    } finally {
        modalPendingRequests = Math.max(0, modalPendingRequests - 1);
    }
}

ensureModalMarkup();
windowForm().addEventListener('submit', handleFormSubmit);
document.addEventListener('keydown', handleKeydown);
window.addEventListener('beforeunload', event => {
    if (modalPendingRequests > 0) {
        event.preventDefault();
        event.returnValue = '';
    }
});

window.openCheckWindowsModal = openCheckWindowsModal;
window.openAddWindow = openAddWindow;
window.openEditWindow = openEditWindow;
window.closeModal = closeModal;
window.cancelForm = cancelForm;
window.deleteWindow = deleteWindow;
