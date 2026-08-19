const API_URL = '';

const pageStatus = document.getElementById('page-status');

let locationGroups = [];

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

function escapeHtml(value) {
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

async function loadLocationGroups() {
    clearPageStatus();
    try {
        const response = await fetch(`${API_URL}/v1/location_groups`);
        if (!response.ok) throw new Error(`failed to load groups (${response.status})`);
        locationGroups = await response.json();
        renderLocationGroups();
    } catch (error) {
        setPageStatus(`Failed to load location groups: ${error.message}`, 'error');
    }
}

function renderLocationGroups() {
    const list = document.getElementById('location-groups-list');
    if (!list) return;
    list.innerHTML = locationGroups
        .map(group => `
            <div class="flex items-center gap-2" data-group-id="${group.id}">
                <input type="text" value="${escapeHtml(group.name)}"
                       class="group-name-input flex-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-slate-500 focus:outline-none"
                       data-group-id="${group.id}" />
                <button class="save-group-button cursor-pointer rounded border border-slate-300 px-2 py-1 text-xs font-medium hover:bg-slate-50"
                        data-group-id="${group.id}">Save</button>
                <button class="delete-group-button cursor-pointer rounded border border-slate-300 px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50"
                        data-group-id="${group.id}">Delete</button>
            </div>`,
        )
        .join('');

    list.querySelectorAll('.save-group-button').forEach(button => {
        button.addEventListener('click', async () => {
            const groupId = Number(button.dataset.groupId);
            const input = list.querySelector(`.group-name-input[data-group-id="${groupId}"]`);
            const name = (input?.value || '').trim();
            if (!name) {
                setPageStatus('Group name cannot be empty.', 'error');
                return;
            }
            try {
                await updateLocationGroup(groupId, name);
                await loadLocationGroups();
                if (pageStatus.classList.contains('hidden')) {
                    setPageStatus('Group renamed.', 'success');
                }
            } catch (error) {
                setPageStatus(`Failed to rename group: ${error.message}`, 'error');
            }
        });
    });

    list.querySelectorAll('.delete-group-button').forEach(button => {
        button.addEventListener('click', async () => {
            const groupId = Number(button.dataset.groupId);
            try {
                await deleteLocationGroup(groupId);
                await loadLocationGroups();
                if (pageStatus.classList.contains('hidden')) {
                    setPageStatus('Group deleted.', 'success');
                }
            } catch (error) {
                if (error.message.includes('in use')) {
                    const group = locationGroups.find(g => g.id === groupId);
                    const groupName = group ? `"${group.name}"` : 'this group';
                    setPageStatus(`Cannot delete ${groupName}: it is assigned to one or more locations or events. Unassign it first, then delete.`, 'error');
                } else {
                    setPageStatus(`Failed to delete group: ${error.message}`, 'error');
                }
            }
        });
    });
}

async function createLocationGroup(name) {
    const response = await fetch(`${API_URL}/v1/admin/location_groups`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
    });
    if (!response.ok) throw new Error(`failed to create group (${response.status})`);
}

async function updateLocationGroup(id, name) {
    const response = await fetch(`${API_URL}/v1/admin/location_groups/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
    });
    if (!response.ok) throw new Error(`failed to rename group (${response.status})`);
}

async function deleteLocationGroup(id) {
    const response = await fetch(`${API_URL}/v1/admin/location_groups/${id}`, { method: 'DELETE' });
    if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || data.sorry || `failed to delete group (${response.status})`);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const addGroupButton = document.getElementById('add-group-button');
    if (addGroupButton) {
        addGroupButton.addEventListener('click', async () => {
            const input = document.getElementById('new-group-name');
            const name = (input?.value || '').trim();
            if (!name) {
                setPageStatus('Group name cannot be empty.', 'error');
                return;
            }
            try {
                await createLocationGroup(name);
                if (input) input.value = '';
                await loadLocationGroups();
                if (pageStatus.classList.contains('hidden')) {
                    setPageStatus('Group added.', 'success');
                }
            } catch (error) {
                setPageStatus(`Failed to add group: ${error.message}`, 'error');
            }
        });
    }

    loadLocationGroups();
});