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

async function syncEvent(eventId) {
    const response = await fetch(`${API_URL}/v1/admin/events`, {
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
        return { event: null, created: false };
    }

    const event = await response.json();
    return { event, created: response.status === 201 };
}

async function getExistingEvent(eventId) {
    const response = await fetch(`${API_URL}/v1/admin/events/lookup?planning_center_id=${encodeURIComponent(eventId)}`);
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
        const locations = await fetchJson(`/v1/admin/planningcenter/events/${eventId}/locations`);
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

const autoFetchModal = () => document.getElementById('auto-fetch-modal');
const autoFetchToggle = () => document.getElementById('auto-fetch-toggle');
const autoFetchToggleBg = () => document.getElementById('auto-fetch-toggle-bg');
const autoFetchToggleKnob = () => document.getElementById('auto-fetch-toggle-knob');
const autoFetchError = () => document.getElementById('auto-fetch-error');
const autoFetchContinue = () => document.getElementById('auto-fetch-continue');

let autoFetchEvent = null;

function setAutoFetchToggleVisual(checked) {
    const bg = autoFetchToggleBg();
    const knob = autoFetchToggleKnob();
    if (bg && knob) {
        bg.style.backgroundColor = checked ? 'var(--color-emerald-500)' : 'var(--color-slate-200)';
        knob.style.transform = checked ? 'translateX(1rem)' : 'translateX(0)';
    }
}

function clearAutoFetchError() {
    const error = autoFetchError();
    if (error) {
        error.classList.add('hidden');
        error.textContent = '';
    }
}

function openAutoFetchModal(event, created = false) {
    const modal = autoFetchModal();
    const toggle = autoFetchToggle();
    if (!modal || !toggle) {
        return;
    }
    autoFetchEvent = event;
    const defaultValue = created ? true : Boolean(event && event.auto_fetch);
    toggle.checked = defaultValue;
    setAutoFetchToggleVisual(toggle.checked);
    clearAutoFetchError();
    modal.classList.remove('hidden');
    modal.setAttribute('aria-hidden', 'false');
}

function closeAutoFetchModal() {
    const modal = autoFetchModal();
    if (modal) {
        modal.classList.add('hidden');
        modal.setAttribute('aria-hidden', 'true');
    }
}

async function handleAutoFetchContinue() {
    const toggle = autoFetchToggle();
    const continueButton = autoFetchContinue();
    if (!toggle || !continueButton || !autoFetchEvent) {
        return;
    }
    const chosenValue = toggle.checked;
    clearAutoFetchError();
    continueButton.disabled = true;
    const originalLabel = continueButton.textContent;
    continueButton.textContent = 'Saving...';
    try {
        if (chosenValue !== autoFetchEvent.auto_fetch) {
            await fetchJson(`/v1/admin/events/${autoFetchEvent.id}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ auto_fetch: chosenValue })
            });
        }
        closeAutoFetchModal();
        if (window.openCheckWindowsModal) {
            window.openCheckWindowsModal(autoFetchEvent.id, autoFetchEvent.name || eventNameLabel.textContent);
        }
    } catch (error) {
        const errorEl = autoFetchError();
        if (errorEl) {
            errorEl.textContent = `Failed to save auto-fetch setting: ${error.message}`;
            errorEl.classList.remove('hidden');
        }
    } finally {
        continueButton.disabled = false;
        continueButton.textContent = originalLabel;
    }
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
        addEventButton.textContent = 'Refresh event data';
        addEventButton.addEventListener('click', async () => {
            addEventButton.disabled = true;
            const originalLabel = addEventButton.textContent;
            addEventButton.textContent = 'Refreshing event...';
            clearPageStatus();
            try {
                const { event, created } = await syncEvent(eventId);
                if (event && event.name) {
                    eventNameLabel.textContent = event.name;
                }
                await loadLocations(eventId);
                setPageStatus('Event data refreshed successfully.', 'success');
                if (eventAddedStatus) {
                    if (created) {
                        eventAddedStatus.classList.add('hidden');
                    } else {
                        eventAddedStatus.classList.remove('hidden');
                    }
                }
                if (event) {
                    openAutoFetchModal(event, created);
                }
            } catch (error) {
                setPageStatus(`Failed to refresh event data: ${error.message}`, 'error');
            } finally {
                addEventButton.disabled = false;
                addEventButton.textContent = originalLabel;
            }
        });
    }

    const autoFetchToggleEl = autoFetchToggle();
    if (autoFetchToggleEl) {
        autoFetchToggleEl.addEventListener('change', () => {
            setAutoFetchToggleVisual(autoFetchToggleEl.checked);
        });
    }
    const continueButton = autoFetchContinue();
    if (continueButton) {
        continueButton.addEventListener('click', handleAutoFetchContinue);
    }

    if (addEventButton) {
        getExistingEvent(eventId)
            .then(event => {
                console.log('[planningcenter-event] lookup result', { eventId, exists: Boolean(event), event });
                if (eventAddedStatus) {
                    if (event) {
                        eventAddedStatus.classList.remove('hidden');
                        if (event.name) {
                            eventNameLabel.textContent = event.name;
                        }
                    } else {
                        eventAddedStatus.classList.add('hidden');
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
