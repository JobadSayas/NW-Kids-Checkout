// API URL
// const API_URL = 'https://demo.visssible.com/backend/get-records.php';

// Store current data
let checkoutData = [];

const locationId = document.getElementById('locationId').dataset.location;

// Function to calculate minutes ago
function calculateMinutesAgo(checkedOutAt) {
    if (!checkedOutAt) return 'just now';

    const checkedOutTime = new Date(checkedOutAt);
    const now = new Date();
    const diffInMinutes = Math.floor((now - checkedOutTime) / (1000 * 60));

    if (diffInMinutes < 1) return 'just now';

    return `${diffInMinutes} min ago`;
}

// Function to update time display for all checkouts
function updateTimes() {
    // Update current checkout time
    if (checkoutData.length > 0) {
        const currentCheckout = checkoutData[0];
        const timeAgo = calculateMinutesAgo(currentCheckout.checked_out_at);
        document.getElementById('current-checkout-time').textContent = timeAgo;
    } else {
        document.getElementById('current-checkout-time').style.display = 'none';
    }

    // Update previously called checkout times
    const timeElements = document.querySelectorAll('.checkout-time');
    timeElements.forEach((element, index) => {
        if (checkoutData[index + 1]) {
            const child = checkoutData[index + 1];
            const timeAgo = calculateMinutesAgo(child.checked_out_at);
            element.textContent = timeAgo;
        }
    });
}

// Function to fetch data from API
async function fetchCheckoutData() {
    try {
        const response = await fetch(encodeURI(`/v1/checkins/checkouts/${locationId}?checked_out_after=-31m&limit=30`));
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();

        // Sort by checked_out_at (most recent first)
        const sortedData = data
            .filter(checkout => checkout.checked_out_at) // Only include children who have been called
            .sort((a, b) => new Date(b.checked_out_at) - new Date(a.checked_out_at));

        checkoutData = sortedData;
        updateUI();
        updateTimes(); // Initialize times

        console.log(`Fetched ${sortedData.length} checkouts`);
    } catch (error) {
        console.error('Error fetching checkout data:', error);
        document.getElementById('current-checkout-name').textContent = 'Error loading data';
        document.getElementById('previously-called-list').innerHTML =
            '<div class="text-center text-red-500 py-8">Error loading data. Please try again.</div>';
    }
}

// Function to update the UI with fetched data
function updateUI() {
    // Update current checkout (most recent)
    if (checkoutData.length > 0) {
        const currentCheckout = checkoutData[0];
        document.getElementById('current-checkout-name').textContent =
            `${currentCheckout.first_name} ${currentCheckout.last_name}`;
        document.getElementById('current-checkout-code').textContent = currentCheckout.security_code;
    } else {
        document.getElementById('current-checkout-name').textContent = 'No checkouts yet';
        document.getElementById('current-checkout-code').textContent = '';
    }

    // Update previously called list (next 7 checkouts)
    const previouslyCalledList = document.getElementById('previously-called-list');
    previouslyCalledList.innerHTML = '';

    // Get next 7 checkouts (excluding the first one which is current)
    const previouslyCalledCheckouts = checkoutData.slice(1, 8);

    if (previouslyCalledCheckouts.length === 0) {
        previouslyCalledList.innerHTML =
            '<div class="text-center text-gray-500 py-8">No previous calls</div>';
        return;
    }

    previouslyCalledCheckouts.forEach(checkout => {
        const card = document.createElement('div');
        card.className = 'bg-white rounded-lg py-3 px-4 shadow-[0_0_10px_rgba(0,0,0,0.25)]';
        card.innerHTML = `
            <div class="font-bold text-gray-800 text-2xl mb-0">
                ${checkout.first_name} ${checkout.last_name}
            </div>
            <div class="flex justify-between items-center">
                <div class="text-black text-xl">
                    ${checkout.security_code}
                </div>
                <div class="text-white bg-gray-400 px-1.5 py-0 rounded-md text-base checkout-time">
                    ${calculateMinutesAgo(checkout.checked_out_at)}
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
        hour12: true,
        hour: '2-digit',
        minute: '2-digit',
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
    // Initial fetch
    fetchCheckoutData();

    // Update the current time immediately and every second
    updateCurrentTime();
    setInterval(updateCurrentTime, 1000);

    // Update minutes ago every 5 seconds
    setInterval(updateTimes, 5000);

    // Fetch new data from API every 5 seconds
    setInterval(fetchCheckoutData, 3000);

    // Update all times every second (for demo/testing)
    setInterval(updateAllTimes, 1000);
});