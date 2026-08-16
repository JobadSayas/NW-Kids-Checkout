// Dev-only asset (see internal/web/dev-assets/README.md for how this is
// served and how to add new dev tools).
//
// Dev helper: loadPreviewData() seeds demo checkouts so you can visually
// validate pill colors and the confirm checkbox without waiting for real time.
// Call from the browser console on the checkouts page: loadPreviewData()
// Use the ?debug param for extra console logging while previewing.
function loadPreviewData() {
    API_CALL_BLOCKS.fetchChildrenData = true;

    const ago = (minutes) => Date.now() - minutes * 60 * 1000;
    childrenData = [
        { first_name: 'Fresh', last_name: 'Kid', security_code: '1111', source: 'planning_center', planning_center_id: 'demo-0', checked_out_at_ms: ago(0), checked_out_confirmed_at: null },
        { first_name: 'Edge', last_name: 'Kid', security_code: '2222', source: 'planning_center', planning_center_id: 'demo-4', checked_out_at_ms: ago(3.9), checked_out_confirmed_at: null },
        { first_name: 'Late', last_name: 'Kid', security_code: '3333', source: 'planning_center', planning_center_id: 'demo-8', checked_out_at_ms: ago(7.9), checked_out_confirmed_at: null },
        { first_name: 'Done', last_name: 'Kid', security_code: '4444', source: 'planning_center', planning_center_id: 'demo-c', checked_out_at_ms: ago(2), checked_out_confirmed_at: '2024-01-01T00:00:00Z' }
    ];

    updateUI();
}