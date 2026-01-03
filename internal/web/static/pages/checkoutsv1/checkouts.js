// API URL
const API_URL = 'http://localhost:3000';

// Store current data
let childrenData = [];

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
    try {
        let params = new URLSearchParams(window.location.search)
        let outParams = new URLSearchParams();

        const limit = params.get('limit')
        if (limit) {
            outParams.append('limit', limit);
        } else {
            outParams.append('limit', '30');
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
    } else {
        document.getElementById('current-child-name').textContent = 'No children called yet';
        document.getElementById('current-child-code').textContent = '----';
    }

    // Update previously called list (next 7 children)
    const previouslyCalledList = document.getElementById('previously-called-list');
    previouslyCalledList.innerHTML = '';

    // Get next 7 children (excluding the first one which is current)
    const previouslyCalledChildren = childrenData.slice(1, 8);

    if (previouslyCalledChildren.length === 0) {
        previouslyCalledList.innerHTML =
            '<div class="text-center text-gray-500 py-8">No previous calls</div>';
        return;
    }

    previouslyCalledChildren.forEach(child => {
        const card = document.createElement('div');
        card.className = 'bg-white rounded-lg py-3 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)]';
        card.innerHTML = `
            <div class="font-bold text-gray-800 text-2xl mb-0">
                ${child.first_name} ${child.last_name}
            </div>
            <div class="flex justify-between items-center">
                <div class="text-black text-xl">
                    ${child.security_code}
                </div>
                <div class="text-white bg-gray-400 px-1.5 py-0 rounded-md text-base child-time">
                    ${calculateMinutesAgo(child.checked_out_at)}
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
    // Initial fetch
    fetchChildrenData();

    // Update current time immediately and every minute
    updateCurrentTime();
    setInterval(updateCurrentTime, 60000);

    // Update minutes ago every minute
    setInterval(updateTimes, 60000);

    // Fetch new data from API every 5 seconds
    setInterval(fetchChildrenData, 5000);

    // Update all times every second (for demo/testing)
    setInterval(updateAllTimes, 1000);
});