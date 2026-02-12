// API URL
const API_URL = '';

// Store current data
let childrenData = [];

const API_CALL_BLOCKS = {
    fetchChildrenData: false,
    confirmCheckedOut: false
};

const CONFIRMED_ICON_SRC = '/static/img/confirmed-checkbox.svg';

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

async function confirmCheckedOut(planningCenterId, checkbox, confirmed) {
    if (!planningCenterId) {
        console.error('Missing planning_center_id for confirmation');
        return;
    }

    if (isApiCallBlocked('confirmCheckedOut')) return;

    API_CALL_BLOCKS.confirmCheckedOut = true;
    try {
        const response = await fetch(
            encodeURI(`${API_URL}/v1/checkins/${planningCenterId}/checked_out_confirmed`),
            {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ confirmed })
            }
        );

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        checkbox.checked = Boolean(confirmed);
    } catch (error) {
        console.error('Error confirming checkout:', error);
    } finally {
        API_CALL_BLOCKS.confirmCheckedOut = false;
    }
}

// Function to calculate minutes ago
function calculateMinutesAgo(checkedOutAt) {
    if (!checkedOutAt) return '0 min ago';

    const checkedOutTime = new Date(checkedOutAt);
    const now = new Date();
    const diffInMinutes = Math.floor((now - checkedOutTime) / (1000 * 60));

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
    const timeElements = document.querySelectorAll('.child-time');
    timeElements.forEach((element, index) => {
        if (childrenData[index + 1]) {
            const child = childrenData[index + 1];
            const timeAgo = calculateMinutesAgo(child.checked_out_at);
            element.textContent = timeAgo;
        }
    });
}

// Function to fetch data from API
async function fetchChildrenData() {
    if (isApiCallBlocked('fetchChildrenData')) return;

    try {
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

        // Sort by checked_out_at (most recent first)
        const sortedData = data
            .filter(child => child.checked_out_at) // Only include children who have been called
            .sort((a, b) => new Date(b.checked_out_at) - new Date(a.checked_out_at));

        childrenData = sortedData;
        updateUI();
        updateTimes(); // Initialize times

        console.log(`Fetched ${sortedData.length} children`);
    } catch (error) {
        console.error('Error fetching children data:', error);
        document.getElementById('current-child-name').textContent = 'Error loading data';
        document.getElementById('previously-called-list').innerHTML =
            '<div class="text-center text-red-500 py-8">Error loading data. Please try again.</div>';
    }
}

// Function to update the UI with fetched data
function updateUI() {
    // Update current child (most recent)
    if (childrenData.length > 0) {
        const currentChild = childrenData[0];
        document.getElementById('current-child-name').textContent =
            `${currentChild.first_name} ${currentChild.last_name}`;
        document.getElementById('current-child-code').textContent = currentChild.security_code;
        const currentConfirmed = Boolean(currentChild.checked_out_confirmed_at);
        const currentConfirmedCheckbox = document.getElementById('current-child-confirmed');
        currentConfirmedCheckbox.checked = currentConfirmed;
        updateConfirmedIcon(currentConfirmedCheckbox);
        if (currentChild.planning_center_id) {
            currentConfirmedCheckbox.dataset.planningCenterId = currentChild.planning_center_id;
        } else {
            delete currentConfirmedCheckbox.dataset.planningCenterId;
        }
    } else {
        document.getElementById('current-child-name').textContent = 'No children called yet';
        document.getElementById('current-child-code').textContent = '----';
        const currentConfirmedCheckbox = document.getElementById('current-child-confirmed');
        currentConfirmedCheckbox.checked = false;
        updateConfirmedIcon(currentConfirmedCheckbox);
        delete currentConfirmedCheckbox.dataset.planningCenterId;
    }

    // Update previously called list (next 7 children)
    const previouslyCalledList = document.getElementById('previously-called-list');
    previouslyCalledList.innerHTML = '';

    // Get next 100 children (excluding the first one which is current)
    const previouslyCalledChildren = childrenData.slice(1, 100);

    if (previouslyCalledChildren.length === 0) {
        previouslyCalledList.innerHTML =
            '<div class="text-center text-gray-500 py-8">No previous calls</div>';
        return;
    }

    previouslyCalledChildren.forEach(child => {
        const card = document.createElement('div');
        card.className = 'bg-white rounded-lg py-2.5 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)] flex flex-col justify-center';
        card.innerHTML = `
            <div class="font-bold text-gray-800 text-2xl mb-0">
                ${child.first_name} ${child.last_name}
            </div>
            <div class="flex justify-between items-center">
                <div class="text-black text-xl">
                    ${child.security_code}
                </div>
                <div class="flex items-center gap-3">
                    <div class="text-white bg-gray-400 px-1.5 py-0 rounded-md text-base child-time">
                        ${calculateMinutesAgo(child.checked_out_at)}
                    </div>
                    <label class="flex items-center text-xs text-gray-600 cursor-pointer leading-none" data-confirmed-label data-confirmed-state="${child.checked_out_confirmed_at ? 'confirmed' : 'unconfirmed'}">
                        <input type="checkbox"
                            class="sr-only child-confirmed-checkbox"
                            data-planning-center-id="${child.planning_center_id || ''}"
                            ${child.checked_out_confirmed_at ? 'checked' : ''}>
                        <img src="${CONFIRMED_ICON_SRC}" alt="" class="h-8 w-8 block" data-confirmed-icon>
                    </label>
                </div>
            </div>
        `;
        previouslyCalledList.appendChild(card);
    });
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
document.addEventListener('DOMContentLoaded', function() {
    document.addEventListener('change', function(event) {
        const checkbox = event.target;
        if (!checkbox.classList.contains('child-confirmed-checkbox')) return;

        const planningCenterId = checkbox.dataset.planningCenterId;
        updateConfirmedIcon(checkbox);
        confirmCheckedOut(planningCenterId, checkbox, checkbox.checked);
    });

    // Initial fetch
    fetchChildrenData();

    // Update current time immediately and every second
    updateCurrentTime();
    setInterval(updateCurrentTime, 1000);

    // Update minutes ago every second
    setInterval(updateTimes, 1000);

    // Fetch new data from API every 3 seconds
    setInterval(fetchChildrenData, 3000);

    // Update all times every second (for demo/testing)
    setInterval(updateAllTimes, 1000);
});
