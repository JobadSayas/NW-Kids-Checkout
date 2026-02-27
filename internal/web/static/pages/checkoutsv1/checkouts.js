// API URL
const API_URL = '';
const DEBUG = new URLSearchParams(window.location.search).has('debug');

// Store current data
let childrenData = [];
let childrenFetchController = null;
let childTimeElementsById = new Map();
let lastCurrentSignature = '';
let lastListSignature = '';
const dom = {
    currentChildName: null,
    currentChildCode: null,
    currentChildTime: null,
    currentChildCard: null,
    previouslyCalledList: null,
    currentTime: null
};

const API_CALL_BLOCKS = {
    fetchChildrenData: false
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
    if (child.source === 'manual') return child.public_id ? `manual:${child.public_id}` : '';
    if (child.source === 'planning_center') return child.planning_center_id ? `pc:${child.planning_center_id}` : '';
    if (child.planning_center_id) return `pc:${child.planning_center_id}`;
    if (child.public_id) return `public:${child.public_id}`;
    return '';
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

function getChildSignature(child) {
    if (!child) return 'empty';
    return [
        child.source || '',
        child.planning_center_id || '',
        child.public_id || '',
        child.checked_out_at || '',
        child.first_name || '',
        child.last_name || '',
        child.security_code || ''
    ].join('|');
}

function syncConfirmedStates() {
    if (!childrenData.length) return;

    const confirmedById = new Map();
    childrenData.forEach((child) => {
        const childId = getChildId(child);
        if (!childId) return;
        confirmedById.set(childId, Boolean(child.checked_out_confirmed_at));
    });

    const roots = [dom.currentChildCard, dom.previouslyCalledList].filter(Boolean);
    roots.forEach((root) => {
        root.querySelectorAll('.child-confirmed-checkbox[data-child-id]').forEach((checkbox) => {
            const childId = checkbox.dataset.childId;
            if (!childId || !confirmedById.has(childId)) return;
            const confirmed = confirmedById.get(childId);
            if (checkbox.checked !== confirmed) {
                checkbox.checked = confirmed;
            }
            const label = checkbox.closest('[data-confirmed-label]');
            if (label) {
                label.dataset.confirmedState = confirmed ? 'confirmed' : 'unconfirmed';
            }
        });
    });
}

function clampPreviouslyCalledScroll() {
    if (!dom.previouslyCalledList) return;
    const maxScrollTop = Math.max(0, dom.previouslyCalledList.scrollHeight - dom.previouslyCalledList.clientHeight);
    if (dom.previouslyCalledList.scrollTop > maxScrollTop) {
        dom.previouslyCalledList.scrollTop = maxScrollTop;
    }
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
        endpoint = `${API_URL}/v1/checkins/manual-checkins/${encodeURIComponent(publicId)}/checked_out_confirmed`;
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
        endpoint = `${API_URL}/v1/checkins/${encodeURIComponent(planningCenterId)}/checked_out_confirmed`;
    }
    checkbox.dataset.confirming = 'true';
    try {
        const response = await fetch(endpoint, {
            method: 'PATCH',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({confirmed})
        });

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
function getCheckedOutTimestamp(value) {
    if (!value) return 0;
    const parsed = Date.parse(value);
    if (Number.isNaN(parsed)) return 0;
    return parsed;
}

function calculateMinutesAgoFromTimestamp(checkedOutAtMs, nowMs) {
    if (!checkedOutAtMs) return '0 min ago';

    const now = typeof nowMs === 'number' ? nowMs : Date.now();
    const diffInMinutes = Math.max(0, Math.floor((now - checkedOutAtMs) / (1000 * 60)));

    return `${diffInMinutes} min ago`;
}

// Function to update time display for all children
function cacheChildTimeElements(container) {
    const timeElements = container.querySelectorAll('.child-time[data-child-id]');
    const nextMap = new Map();
    timeElements.forEach((element) => {
        const id = element.dataset.childId;
        if (id) nextMap.set(id, element);
    });
    childTimeElementsById = nextMap;
}

function updateTimes() {
    if (!childrenData.length) {
        return;
    }
    const nowMs = Date.now();
    // Update current child time
    const currentChild = childrenData[0];
    const checkedOutAtMs = currentChild.checked_out_at_ms ?? getCheckedOutTimestamp(currentChild.checked_out_at);
    const timeAgo = calculateMinutesAgoFromTimestamp(checkedOutAtMs, nowMs);
    if (!dom.currentChildTime) {
        dom.currentChildTime = document.getElementById('current-child-time');
    }
    if (dom.currentChildTime && dom.currentChildTime.textContent !== timeAgo) {
        dom.currentChildTime.textContent = timeAgo;
    }

    // Update previously called children times
    childrenData.slice(1, 100).forEach((child) => {
        const id = getChildId(child);
        const element = childTimeElementsById.get(id);
        if (!element) return;
        const checkedOutAtMs = child.checked_out_at_ms ?? getCheckedOutTimestamp(child.checked_out_at);
        const nextValue = calculateMinutesAgoFromTimestamp(checkedOutAtMs, nowMs);
        if (element.textContent !== nextValue) {
            element.textContent = nextValue;
        }
    });
}

// Function to fetch data from API
async function fetchChildrenData() {
    if (isApiCallBlocked('fetchChildrenData')) return;

    let controller = null;
    try {
        API_CALL_BLOCKS.fetchChildrenData = true;
        if (childrenFetchController) {
            childrenFetchController.abort();
        }
        controller = new AbortController();
        childrenFetchController = controller;
        let params = new URLSearchParams(window.location.search)
        let outParams = new URLSearchParams();

        const limit = params.get('limit')
        if (limit) {
            outParams.append('limit', limit);
        } else {
            outParams.append('limit', '100');
        }

        const locationGroupName = params.get('location_group_name')
        if (locationGroupName) outParams.append('location_group_name', locationGroupName);

        const locationGroupId = params.get('location_group_id')
        if (locationGroupId) outParams.append('location_group_id', locationGroupId);

        const checkedOutAfter = params.get('checked_out_after')
        if (checkedOutAfter) outParams.append('checked_out_after', checkedOutAfter);

        const response = await fetch(`${API_URL}/v1/checkins/checkouts/?${outParams.toString()}`, {
            signal: controller.signal
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        const combined = normalizeCheckoutsResponse(data).map((child) => {
            const checkedOutAtMs = getCheckedOutTimestamp(child.checked_out_at);
            return {
                ...child,
                checked_out_at_ms: checkedOutAtMs
            };
        });

        // Sort by checked_out_at (most recent first)
        const sortedData = combined
            .filter(child => child.checked_out_at_ms) // Only include children who have been called
            .sort((a, b) => b.checked_out_at_ms - a.checked_out_at_ms);

        childrenData = sortedData;
        updateUI();
        updateTimes(); // Initialize times

        if (DEBUG) {
            console.log(`Fetched ${sortedData.length} children`);
        }
    } catch (error) {
        if (error?.name === 'AbortError') return;
        console.error('Error fetching children data:', error);
        childrenData = [];
        lastCurrentSignature = '';
        lastListSignature = '';
        if (!dom.currentChildName) {
            dom.currentChildName = document.getElementById('current-child-name');
        }
        if (dom.currentChildName) {
            dom.currentChildName.textContent = 'Error loading data';
        }
        if (!dom.currentChildCode) {
            dom.currentChildCode = document.getElementById('current-child-code');
        }
        if (dom.currentChildCode) {
            dom.currentChildCode.textContent = '----';
        }
        if (!dom.currentChildTime) {
            dom.currentChildTime = document.getElementById('current-child-time');
        }
        if (dom.currentChildTime) {
            dom.currentChildTime.textContent = '0 min ago';
        }
        if (!dom.previouslyCalledList) {
            dom.previouslyCalledList = document.getElementById('previously-called-list');
        }
        if (dom.previouslyCalledList) {
            dom.previouslyCalledList.innerHTML =
                '<div class="text-center text-red-500 py-8">Error loading data. Please try again.</div>';
            dom.previouslyCalledList.scrollTop = 0;
        }
    } finally {
        API_CALL_BLOCKS.fetchChildrenData = false;
        if (childrenFetchController === controller) {
            childrenFetchController = null;
        }
    }
}

// Function to update the UI with fetched data
function updateUI() {
    if (!dom.currentChildCard) {
        dom.currentChildCard = document.getElementById('current-child-card');
    }
    if (!dom.previouslyCalledList) {
        dom.previouslyCalledList = document.getElementById('previously-called-list');
    }

    const nowMs = Date.now();
    const currentSignature = getChildSignature(childrenData[0]);
    if (dom.currentChildCard && currentSignature !== lastCurrentSignature) {
        const currentMarkup = renderCurrentChild(childrenData[0], nowMs);
        morphChildren(dom.currentChildCard, currentMarkup);
        lastCurrentSignature = currentSignature;
        dom.currentChildTime = document.getElementById('current-child-time');
    }

    const listSignature = childrenData
        .slice(1, 100)
        .map(getChildSignature)
        .join('||');
    if (dom.previouslyCalledList && listSignature !== lastListSignature) {
        const previousScrollTop = dom.previouslyCalledList.scrollTop;
        const previouslyCalledChildren = childrenData.slice(1, 100);
        const listMarkup = renderPreviouslyCalled(previouslyCalledChildren, nowMs);
        morphChildren(dom.previouslyCalledList, listMarkup);
        cacheChildTimeElements(dom.previouslyCalledList);
        requestAnimationFrame(() => {
            if (!dom.previouslyCalledList) return;
            const maxScrollTop = Math.max(0, dom.previouslyCalledList.scrollHeight - dom.previouslyCalledList.clientHeight);
            dom.previouslyCalledList.scrollTop = Math.min(previousScrollTop, maxScrollTop);
        });
        lastListSignature = listSignature;
    }
    syncConfirmedStates();
}

function renderCurrentChild(child, nowMs) {
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
                <label class="relative flex items-center cursor-pointer leading-none" data-confirmed-label data-confirmed-state="unconfirmed">
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
    const checkedOutAtMs = child.checked_out_at_ms ?? getCheckedOutTimestamp(child.checked_out_at);
    const childId = escapeHtml(getChildId(child));

    return `
        <div id="current-child-name" class="font-bold text-4xl lg:text-6xl lg:text-7xl text-gray-800 mb-1 lg:mb-3">
            ${name}${starMarkup}
        </div>
        <div class="flex items-center gap-4">
            <div id="current-child-code" class="text-3xl lg:text-6xl text-black mr-4">
                ${code}
            </div>
            <div id="current-child-time" class="text-xl lg:text-xl text-white bg-gray-400 px-2 py-0 lg:px-2 lg:py-1 rounded-md">
                ${calculateMinutesAgoFromTimestamp(checkedOutAtMs, nowMs)}
            </div>
            <label class="relative flex items-center cursor-pointer leading-none" data-confirmed-label data-confirmed-state="${confirmed ? 'confirmed' : 'unconfirmed'}">
                <input id="current-child-confirmed" type="checkbox" class="sr-only child-confirmed-checkbox"
                    data-child-id="${childId}" data-planning-center-id="${planningCenterId}" data-public-id="${publicId}" data-source="${source}" ${confirmed ? 'checked' : ''}>
                <span class="inline-flex">
                    <img src="${CONFIRMED_ICON_SRC}" alt="" class="h-10 w-10 block" data-confirmed-icon>
                </span>
            </label>
        </div>
    `;
}

function renderPreviouslyCalled(children, nowMs) {
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
        const checkedOutAtMs = child.checked_out_at_ms ?? getCheckedOutTimestamp(child.checked_out_at);

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
                            ${calculateMinutesAgoFromTimestamp(checkedOutAtMs, nowMs)}
                        </div>
                        <label class="relative flex items-center text-xs text-gray-600 cursor-pointer leading-none" data-confirmed-label data-confirmed-state="${confirmed ? 'confirmed' : 'unconfirmed'}">
                            <input type="checkbox" class="sr-only child-confirmed-checkbox"
                                data-child-id="${childId}" data-planning-center-id="${planningCenterId}" data-public-id="${publicId}" data-source="${source}" ${confirmed ? 'checked' : ''}>
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
    if (!dom.currentTime) {
        dom.currentTime = document.getElementById('current-time');
    }
    if (!dom.currentTime) {
        return;
    }
    const now = new Date();
    const timeString = now.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
    });
    dom.currentTime.textContent = timeString;
}

// Function to update all times (current time and minutes ago)
function updateAllTimes() {
    updateCurrentTime();
    updateTimes();
}

// Initialize and start periodic updates
document.addEventListener('DOMContentLoaded', function () {
    dom.currentChildCard = document.getElementById('current-child-card');
    dom.previouslyCalledList = document.getElementById('previously-called-list');
    dom.currentChildName = document.getElementById('current-child-name');
    dom.currentChildCode = document.getElementById('current-child-code');
    dom.currentTime = document.getElementById('current-time');
    dom.currentChildTime = document.getElementById('current-child-time');

    window.addEventListener('resize', () => {
        requestAnimationFrame(clampPreviouslyCalledScroll);
    });

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
