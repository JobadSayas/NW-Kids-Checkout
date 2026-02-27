function sortLocationsByPlanningCenterHierarchy(items) {
    const locationKeys = new Map();
    const byId = new Map();
    const childrenByParent = new Map();
    const roots = [];

    items.forEach((location, index) => {
        const id = String(location.planning_center_id || '').trim();
        locationKeys.set(location, id ? `pc:${id}` : `idx:${index}`);
        if (id) {
            byId.set(id, location);
        }
    });

    items.forEach(location => {
        const parentId = location.planning_center_parent_id ? String(location.planning_center_parent_id).trim() : '';
        if (!parentId) {
            roots.push(location);
            return;
        }

        if (!byId.has(parentId)) {
            roots.push(location);
            return;
        }

        if (!childrenByParent.has(parentId)) {
            childrenByParent.set(parentId, []);
        }
        childrenByParent.get(parentId).push(location);
    });

    const compareByName = (left, right) => {
        const leftName = left.name ? left.name.toLowerCase() : '';
        const rightName = right.name ? right.name.toLowerCase() : '';
        const nameCompare = leftName.localeCompare(rightName);
        if (nameCompare !== 0) {
            return nameCompare;
        }
        return String(left.planning_center_id || '').localeCompare(String(right.planning_center_id || ''));
    };

    const compareByPlanningCenterID = (left, right) => {
        const leftId = String(left.planning_center_id || '').trim().toLowerCase();
        const rightId = String(right.planning_center_id || '').trim().toLowerCase();
        const idCompare = leftId.localeCompare(rightId);
        if (idCompare !== 0) {
            return idCompare;
        }
        const leftName = left.name ? left.name.toLowerCase() : '';
        const rightName = right.name ? right.name.toLowerCase() : '';
        return leftName.localeCompare(rightName);
    };

    const compareByEventThenPlanningCenterID = (left, right) => {
        const leftEvent = left.event_id;
        const rightEvent = right.event_id;
        const leftMissing = leftEvent === null || leftEvent === undefined || leftEvent === '';
        const rightMissing = rightEvent === null || rightEvent === undefined || rightEvent === '';

        if (leftMissing && rightMissing) {
            return compareByPlanningCenterID(left, right);
        }
        if (leftMissing) {
            return 1;
        }
        if (rightMissing) {
            return -1;
        }

        const leftNumber = Number(leftEvent);
        const rightNumber = Number(rightEvent);
        const leftNumeric = Number.isFinite(leftNumber);
        const rightNumeric = Number.isFinite(rightNumber);
        if (leftNumeric && rightNumeric) {
            const eventCompare = leftNumber - rightNumber;
            if (eventCompare !== 0) {
                return eventCompare;
            }
            return compareByPlanningCenterID(left, right);
        }

        const eventCompare = String(leftEvent).toLowerCase().localeCompare(String(rightEvent).toLowerCase());
        if (eventCompare !== 0) {
            return eventCompare;
        }
        return compareByPlanningCenterID(left, right);
    };

    const result = [];
    const visited = new Set();

    const appendBranch = location => {
        const key = locationKeys.get(location);
        const id = String(location.planning_center_id || '').trim();
        if (!key || visited.has(key)) {
            return;
        }
        visited.add(key);
        result.push(location);

        const children = childrenByParent.get(id) || [];
        children.sort(compareByName);
        children.forEach(child => appendBranch(child));
    };

    roots.sort(compareByEventThenPlanningCenterID);
    roots.forEach(root => appendBranch(root));

    const remaining = items.filter(location => {
        const key = locationKeys.get(location);
        return key && !visited.has(key);
    });
    remaining.sort(compareByEventThenPlanningCenterID);
    remaining.forEach(location => appendBranch(location));

    return result;
}

if (typeof globalThis !== 'undefined') {
    if (!globalThis.NWKidsLocationSort) {
        globalThis.NWKidsLocationSort = {};
    }
    globalThis.NWKidsLocationSort.sortLocationsByPlanningCenterHierarchy = sortLocationsByPlanningCenterHierarchy;
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = {sortLocationsByPlanningCenterHierarchy};
}
