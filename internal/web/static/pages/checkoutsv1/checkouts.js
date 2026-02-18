// API URL
const API_URL = '';
const DEBUG = new URLSearchParams(window.location.search).has('debug');

// Store current data
let childrenData = [];

const API_CALL_BLOCKS = {
    fetchChildrenData: false,
    confirmCheckedOut: false
};

const CONFIRMED_ICON_SRC = '/static/img/confirmed-checkbox.svg';
const MANUAL_STAR_ICON_SRC = '/static/img/star.svg';

function escapeHtml(value) {
    return String(value ?? '').replace(/[&<>"']/g, (character) => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
    }[character]));
}

function getManualCheckinStarMarkup(source) {
    if (source !== 'manual') return '';
    return ` <img src="${MANUAL_STAR_ICON_SRC}" alt="Manual checkin" class="inline-block h-5 w-5 ml-2 relative -top-0.5">`;
}

function getChildId(child) {
    if (!child) return '';
    if (child.source === 'manual' && child.public_id) return `manual:${child.public_id}`;
    if (child.planning_center_id) return `pc:${child.planning_center_id}`;
    if (child.public_id) return `public:${child.public_id}`;
    return `row:${child.first_name || ''}-${child.last_name || ''}-${child.checked_out_at || ''}`;
}

function normalizeCheckoutsResponse(data) {
    if (Array.isArray(data)) return data;

    const normalizeList = (value) => {
        if (Array.isArray(value)) return value;
        if (Array.isArray(value?.checkins)) return value.checkins;
        return [];
    };

    const checkins = normalizeList(data?.checkins);
    const manualCheckins = normalizeList(data?.manual_checkins);
    return [...checkins, ...manualCheckins];
}

function updateConfirmedIcon(checkbox) {
    const icon = checkbox.closest('label')?.querySelector('[data-confirmed-icon]');
    if (!icon) return;

    const label = checkbox.closest('[data-confirmed-label]');
    if (label) {
        label.dataset.confirmedState = checkbox.checked ? 'confirmed' : 'unconfirmed';
    }
}

function isApiCallBlocked(callName) {
    return Boolean(API_CALL_BLOCKS[callName]);
}

async function confirmCheckedOut(source, planningCenterId, publicId, checkbox, confirmed, previousConfirmed) {
    if (checkbox.dataset.confirming === 'true') return;
    let endpoint = '';
    if (source === 'manual') {
        if (!publicId) {
            console.error('Missing public_id for manual confirmation');
            checkbox.checked = previousConfirmed;
            updateConfirmedIcon(checkbox);
            return;
        }
        endpoint = `${API_URL}/v1/checkins/manual-checkins/${publicId}/checked_out_confirmed`;
    } else {
        if (source && source !== 'planning_center') {
            console.warn(`Skipping confirmation for source: ${source}`);
            checkbox.checked = previousConfirmed;
            updateConfirmedIcon(checkbox);
            return;
        }
        if (!planningCenterId) {
            console.error('Missing planning_center_id for confirmation');
            checkbox.checked = previousConfirmed;
            updateConfirmedIcon(checkbox);
            return;
        }
        endpoint = `${API_URL}/v1/checkins/${planningCenterId}/checked_out_confirmed`;
    }
    checkbox.dataset.confirming = 'true';
    try {
        const response = await fetch(
            encodeURI(endpoint),
            {
                method: 'PATCH',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({confirmed})
            }
        );

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        checkbox.checked = Boolean(confirmed);
    } catch (error) {
        console.error('Error confirming checkout:', error);
        checkbox.checked = previousConfirmed;
        updateConfirmedIcon(checkbox);
    } finally {
        delete checkbox.dataset.confirming;
    }
}

// Function to calculate minutes ago
function calculateMinutesAgo(checkedOutAt) {
    if (!checkedOutAt) return '0 min ago';

    const checkedOutTime = new Date(checkedOutAt);
    const now = new Date();
    if (Number.isNaN(checkedOutTime.getTime())) return '0 min ago';
    const diffInMinutes = Math.max(0, Math.floor((now - checkedOutTime) / (1000 * 60)));

    return `${diffInMinutes} min ago`;
}

// Function to update time display for all children
function updateTimes() {
    // Update current child time
    if (childrenData.length > 0) {
        const currentChild = childrenData[0];
        const timeAgo = calculateMinutesAgo(currentChild.checked_out_at);
        document.getElementById('current-child-time').textContent = timeAgo;
    }

    // Update previously called children times
    const timeElements = document.querySelectorAll('.child-time[data-child-id]');
    const timeById = new Map();
    timeElements.forEach((element) => {
        const id = element.dataset.childId;
        if (id) timeById.set(id, element);
    });
    childrenData.slice(1, 100).forEach((child) => {
        const id = getChildId(child);
        const element = timeById.get(id);
        if (!element) return;
        element.textContent = calculateMinutesAgo(child.checked_out_at);
    });
}

// Function to fetch data from API
async function fetchChildrenData() {
    if (isApiCallBlocked('fetchChildrenData')) return;

    try {
        API_CALL_BLOCKS.fetchChildrenData = true;
        let params = new URLSearchParams(window.location.search)
        let outParams = new URLSearchParams();

        const limit = params.get('limit')
        if (limit) {
            outParams.append('limit', limit);
        } else {
            outParams.append('limit', '100');
        }

        const locationGroupName = params.get('location_group_name')
        if (locationGroupName) outParams.append('location_group_name', decodeURI(locationGroupName));

        const locationGroupId = params.get('location_group_id')
        if (locationGroupId) outParams.append('location_group_id', locationGroupId);

        const checkedOutAfter = params.get('checked_out_after')
        if (checkedOutAfter) outParams.append('checked_out_after', checkedOutAfter);

        const response = await fetch(encodeURI(`${API_URL}/v1/checkins/checkouts/?${outParams.toString()}`));
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const combined = normalizeCheckoutsResponse(data);

        // Sort by checked_out_at (most recent first)
        const sortedData = combined
            .filter(child => child.checked_out_at) // Only include children who have been called
            .sort((a, b) => new Date(b.checked_out_at) - new Date(a.checked_out_at));

        childrenData = sortedData;
        updateUI();
        updateTimes(); // Initialize times

        if (DEBUG) {
            console.log(`Fetched ${sortedData.length} children`);
        }
    } catch (error) {
        console.error('Error fetching children data:', error);
        document.getElementById('current-child-name').textContent = 'Error loading data';
        document.getElementById('previously-called-list').innerHTML =
            '<div class="text-center text-red-500 py-8">Error loading data. Please try again.</div>';
    } finally {
        API_CALL_BLOCKS.fetchChildrenData = false;
    }
}

// Function to update the UI with fetched data
function updateUI() {
    const currentCard = document.getElementById('current-child-card');
    const currentMarkup = renderCurrentChild(childrenData[0]);
    morphChildren(currentCard, currentMarkup);

    const previouslyCalledList = document.getElementById('previously-called-list');
    const previouslyCalledChildren = childrenData.slice(1, 100);
    const listMarkup = renderPreviouslyCalled(previouslyCalledChildren);
    morphChildren(previouslyCalledList, listMarkup);
}

function renderCurrentChild(child) {
    if (!child) {
        return `
            <div id="current-child-name" class="font-bold text-4xl lg:text-6xl lg:text-7xl text-gray-800 mb-1 lg:mb-3">
                No children called yet
            </div>
            <div class="flex items-center gap-4">
                <div id="current-child-code" class="text-3xl lg:text-6xl text-black mr-4">
                    ----
                </div>
                <div id="current-child-time" class="text-xl lg:text-xl text-white bg-gray-400 px-2 py-0 lg:px-2 lg:py-1 rounded-md">
                    0 min ago
                </div>
                <label class="flex items-center cursor-pointer leading-none" data-confirmed-label data-confirmed-state="unconfirmed">
                    <input id="current-child-confirmed" type="checkbox" class="sr-only child-confirmed-checkbox">
                    <span class="inline-flex">
                        <img src="${CONFIRMED_ICON_SRC}" alt="" class="h-10 w-10 block" data-confirmed-icon>
                    </span>
                </label>
            </div>
        `;
    }

    const name = `${escapeHtml(child.first_name)} ${escapeHtml(child.last_name)}`;
    const code = child.source === 'manual' ? '---' : escapeHtml(child.security_code || '----');
    const confirmed = Boolean(child.checked_out_confirmed_at);
    const planningCenterId = escapeHtml(child.planning_center_id || '');
    const publicId = escapeHtml(child.public_id || '');
    const source = escapeHtml(child.source || '');
    const starMarkup = getManualCheckinStarMarkup(child.source);

    return `
        <div id="current-child-name" class="font-bold text-4xl lg:text-6xl lg:text-7xl text-gray-800 mb-1 lg:mb-3">
            ${name}${starMarkup}
        </div>
        <div class="flex items-center gap-4">
            <div id="current-child-code" class="text-3xl lg:text-6xl text-black mr-4">
                ${code}
            </div>
            <div id="current-child-time" class="text-xl lg:text-xl text-white bg-gray-400 px-2 py-0 lg:px-2 lg:py-1 rounded-md">
                ${calculateMinutesAgo(child.checked_out_at)}
            </div>
            <label class="flex items-center cursor-pointer leading-none" data-confirmed-label data-confirmed-state="${confirmed ? 'confirmed' : 'unconfirmed'}">
                <input id="current-child-confirmed" type="checkbox" class="sr-only child-confirmed-checkbox"
                    data-planning-center-id="${planningCenterId}" data-public-id="${publicId}" data-source="${source}" ${confirmed ? 'checked' : ''}>
                <span class="inline-flex">
                    <img src="${CONFIRMED_ICON_SRC}" alt="" class="h-10 w-10 block" data-confirmed-icon>
                </span>
            </label>
        </div>
    `;
}

function renderPreviouslyCalled(children) {
    if (children.length === 0) {
        return '<div class="text-center text-gray-500 py-8">No previous calls</div>';
    }

    return children.map((child) => {
        const name = `${escapeHtml(child.first_name)} ${escapeHtml(child.last_name)}`;
        const code = child.source === 'manual' ? '---' : escapeHtml(child.security_code || '----');
        const confirmed = Boolean(child.checked_out_confirmed_at);
        const planningCenterId = escapeHtml(child.planning_center_id || '');
        const publicId = escapeHtml(child.public_id || '');
        const source = escapeHtml(child.source || '');
        const starMarkup = getManualCheckinStarMarkup(child.source);
        const childId = escapeHtml(getChildId(child));

        return `
            <div class="bg-white rounded-lg py-2.5 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)] flex flex-col justify-center">
                <div class="font-bold text-gray-800 text-2xl mb-0">
                    ${name}${starMarkup}
                </div>
                <div class="flex justify-between items-center">
                    <div class="text-black text-xl">
                        ${code}
                    </div>
                    <div class="flex items-center gap-3">
                        <div class="text-white bg-gray-400 px-1.5 py-0 rounded-md text-base child-time" data-child-id="${childId}">
                            ${calculateMinutesAgo(child.checked_out_at)}
                        </div>
                        <label class="flex items-center text-xs text-gray-600 cursor-pointer leading-none" data-confirmed-label data-confirmed-state="${confirmed ? 'confirmed' : 'unconfirmed'}">
                            <input type="checkbox" class="sr-only child-confirmed-checkbox"
                                data-planning-center-id="${planningCenterId}" data-public-id="${publicId}" data-source="${source}" ${confirmed ? 'checked' : ''}>
                            <img src="${CONFIRMED_ICON_SRC}" alt="" class="h-8 w-8 block" data-confirmed-icon>
                        </label>
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

function morphChildren(target, html) {
    const template = document.createElement('div');
    template.innerHTML = html;
    if (DEBUG) {
        const start = performance.now();
        morphdom(target, template, {childrenOnly: true});
        const end = performance.now();
        console.log(`[morphdom] ${target.id || target.className || target.tagName} updated in ${(end - start).toFixed(2)}ms`);
        return;
    }
    morphdom(target, template, {childrenOnly: true});
}

// Function to update current time display
function updateCurrentTime() {
    const now = new Date();
    const timeString = now.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
    });
    document.getElementById('current-time').textContent = timeString;
}

// Function to update all times (current time and minutes ago)
function updateAllTimes() {
    updateCurrentTime();
    updateTimes();
}

// Initialize and start periodic updates
document.addEventListener('DOMContentLoaded', function () {
    document.addEventListener('change', function (event) {
        const checkbox = event.target;
        if (!checkbox.classList.contains('child-confirmed-checkbox')) return;

        const planningCenterId = checkbox.dataset.planningCenterId;
        const publicId = checkbox.dataset.publicId;
        const source = checkbox.dataset.source;
        const label = checkbox.closest('[data-confirmed-label]');
        const previousConfirmed = label?.dataset.confirmedState === 'confirmed';
        updateConfirmedIcon(checkbox);
        confirmCheckedOut(source, planningCenterId, publicId, checkbox, checkbox.checked, previousConfirmed);
    });

    // Initial fetch
    fetchChildrenData();

    // Fetch new data from API every 3 seconds
    setInterval(fetchChildrenData, 3000);

    updateAllTimes();
    setInterval(updateAllTimes, 1000);
});
