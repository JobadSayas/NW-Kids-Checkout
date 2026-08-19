import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/checkoutsv1/checkouts.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const exposeInternals = `
window.__test = {
    setChildrenData: (value) => { childrenData = value; },
    setDom: () => {
        dom.childrenList = document.getElementById('children-list');
    },
    syncConfirmedStates: () => syncConfirmedStates(),
    setConfirmationOverride: (childId, confirmed) => setConfirmationOverride(childId, confirmed),
    setSearchQuery: (query) => setSearchQuery(query),
    getVisibleChildren: () => getVisibleChildren()
};
`;

function loadWindow({ html, url = 'http://localhost/', fetchImpl } = {}) {
    const dom = new JSDOM(html || '<!doctype html><html><body></body></html>', {
        runScripts: 'dangerously',
        url
    });
    dom.window.fetch = fetchImpl || (async () => ({
        ok: true,
        json: async () => [],
        text: async () => ''
    }));
    dom.window.setInterval = () => 0;
    dom.window.morphdom = (target, template) => {
        target.innerHTML = template.innerHTML;
    };
    dom.window.requestAnimationFrame = () => 0;
    dom.window.eval(`${script}\n${exposeInternals}`);
    return dom.window;
}

describe('checkoutsv1/checkouts', () => {
    it('exposes core helper functions', () => {
        const window = loadWindow();
        expect(typeof window.getChildId).toBe('function');
        expect(typeof window.normalizeCheckoutsResponse).toBe('function');
        expect(typeof window.getCheckedOutTimestamp).toBe('function');
        expect(typeof window.calculateMinutesAgoFromTimestamp).toBe('function');
        expect(typeof window.renderChildren).toBe('function');
    });

    it('builds child ids by source', () => {
        const window = loadWindow();
        expect(window.getChildId({ source: 'manual', public_id: '123' })).toBe('manual:123');
        expect(window.getChildId({ source: 'manual' })).toBe('');
        expect(window.getChildId({ source: 'planning_center', planning_center_id: 'pc-1' })).toBe('pc:pc-1');
        expect(window.getChildId({ planning_center_id: 'pc-2' })).toBe('pc:pc-2');
        expect(window.getChildId({ public_id: 'pub-1' })).toBe('public:pub-1');
    });

    it('normalizes checkout payloads into a single list', () => {
        const window = loadWindow();
        const combined = window.normalizeCheckoutsResponse({
            checkins: [{ id: 'a' }],
            manual_checkins: [{ id: 'b' }]
        });
        expect(combined).toEqual([{ id: 'a' }, { id: 'b' }]);

        const nested = window.normalizeCheckoutsResponse({
            checkins: { checkins: [{ id: 'c' }] },
            manual_checkins: { checkins: [{ id: 'd' }] }
        });
        expect(nested).toEqual([{ id: 'c' }, { id: 'd' }]);
    });

    it('parses timestamps and formats minutes ago', () => {
        const window = loadWindow();
        const ts = window.getCheckedOutTimestamp('2024-01-01T00:00:00Z');
        expect(ts).toBe(Date.parse('2024-01-01T00:00:00Z'));
        expect(window.getCheckedOutTimestamp('not-a-date')).toBe(0);
        expect(window.calculateMinutesAgoFromTimestamp(0, 123)).toBe('0 min ago');
        expect(window.calculateMinutesAgoFromTimestamp(2 * 60 * 1000, 5 * 60 * 1000)).toBe('3 min ago');
    });

    it('renders children with escaped text, ids, manual star, and timing', () => {
        const window = loadWindow();
        const html = window.renderChildren([
            {
                first_name: '<Ada>',
                last_name: 'Lovelace',
                security_code: '1234',
                checked_out_at_ms: 3 * 60 * 1000,
                checked_out_confirmed_at: '2024-01-01T00:05:00Z',
                planning_center_id: 'pc-1',
                public_id: 'pub-1',
                source: 'manual'
            },
            {
                first_name: 'Sam',
                last_name: '<Test>',
                security_code: '9999',
                checked_out_at_ms: 60 * 1000,
                planning_center_id: '11',
                source: 'planning_center'
            }
        ], 5 * 60 * 1000);

        expect(html).toContain('&lt;Ada&gt;');
        expect(html).toContain('/static/img/star.svg');
        expect(html).toContain('data-confirmed-state="confirmed"');
        expect(html).toContain('data-child-id="manual:pub-1"');
        expect(html).toContain('---');
        expect(html).toContain('data-child-id="pc:11"');
        expect(html).toContain('&lt;Test&gt;');
        expect(html).toContain('2 min ago');
    });

    it('steps pill color green to yellow at 4 min and red at 8 min', () => {
        const window = loadWindow();
        const base = 1000;
        const at = (minutes) => base + minutes * 60 * 1000;
        expect(window.getTimePillClass(base, false, at(0))).toBe('bg-green-500');
        expect(window.getTimePillClass(base, false, at(3))).toBe('bg-green-500');
        expect(window.getTimePillClass(base, false, at(4))).toBe('bg-yellow-500');
        expect(window.getTimePillClass(base, false, at(7))).toBe('bg-yellow-500');
        expect(window.getTimePillClass(base, false, at(8))).toBe('bg-red-500');
        expect(window.getTimePillClass(base, false, at(30))).toBe('bg-red-500');
    });

    it('uses gray for confirmed checkouts and green when no timestamp', () => {
        const window = loadWindow();
        expect(window.getTimePillClass(0, true, 30 * 60 * 1000)).toBe('bg-gray-400');
        expect(window.getTimePillClass(0, true, 0)).toBe('bg-gray-400');
        expect(window.getTimePillClass(0, false, Date.now())).toBe('bg-green-500');
    });

    it('swaps the pill class when confirmed state changes', () => {
        const html = `<!doctype html>
            <html>
                <body>
                    <div id="children-list">
                        <div class="child-time bg-green-500" data-child-id="pc:11">0 min ago</div>
                    </div>
                </body>
            </html>`;
        const window = loadWindow({ html });
        const pill = window.document.querySelector('.child-time');
        const child = {
            source: 'planning_center',
            planning_center_id: '11',
            checked_out_at_ms: 1000,
            checked_out_confirmed_at: null
        };

        window.applyPillColor(pill, child, true, Date.now());
        expect(pill.className).toContain('bg-gray-400');
        expect(pill.className).not.toContain('bg-green-500');

        window.applyPillColor(pill, child, false, 1000);
        expect(pill.className).toContain('bg-green-500');
        expect(pill.className).not.toContain('bg-gray-400');
    });

    it('renders pill with stepped background class', () => {
        const window = loadWindow();
        const html = window.renderChildren([
            {
                first_name: 'Ada',
                last_name: 'Lovelace',
                security_code: '1234',
                checked_out_at_ms: 3 * 60 * 1000,
                checked_out_confirmed_at: '2024-01-01T00:05:00Z',
                planning_center_id: 'pc-1',
                source: 'planning_center'
            },
            {
                first_name: 'Sam',
                last_name: 'Test',
                security_code: '9999',
                checked_out_at_ms: 60 * 1000,
                planning_center_id: '11',
                source: 'planning_center'
            }
        ], 5 * 60 * 1000);

        expect(html).toContain('bg-gray-400');
        expect(html).toContain('bg-yellow-500');
        expect(html).toContain('transition-colors');
        expect(html).not.toContain('background-color:');
    });

    it('renders empty state when no checkouts are active', () => {
        const window = loadWindow();
        const html = window.renderChildren([], Date.now());
        expect(html).toContain('No children called yet');
    });

    it('shows error state in children list on fetch error', async () => {
        const html = `<!doctype html>
            <html>
                <body>
                    <div id="children-list">
                        <div class="child-time" data-child-id="pc:1">5 min ago</div>
                    </div>
                </body>
            </html>`;
        const window = loadWindow({
            html,
            fetchImpl: async () => {
                throw new Error('offline');
            }
        });

        const originalConsoleError = window.console.error;
        window.console.error = () => { };

        try {
            await window.fetchChildrenData();
        } finally {
            window.console.error = originalConsoleError;
        }

        expect(window.document.getElementById('children-list').innerHTML)
            .toContain('Error loading data. Please try again.');
    });

    it('keeps confirmation overrides applied until data catches up', () => {
        const html = `<!doctype html>
            <html>
                <body>
                    <div id="children-list">
                        <label data-confirmed-label data-confirmed-state="unconfirmed">
                            <input type="checkbox" class="child-confirmed-checkbox" data-child-id="manual:pub-1">
                        </label>
                    </div>
                </body>
            </html>`;
        const window = loadWindow({ html });

        window.__test.setDom();
        window.__test.setChildrenData([
            {
                source: 'manual',
                public_id: 'pub-1',
                checked_out_confirmed_at: null,
                checked_out_at: '2024-01-01T00:00:00Z'
            }
        ]);

        window.__test.setConfirmationOverride('manual:pub-1', true);
        window.__test.syncConfirmedStates();

        const checkboxes = window.document.querySelectorAll('.child-confirmed-checkbox');
        checkboxes.forEach((checkbox) => {
            expect(checkbox.checked).toBe(true);
            const label = checkbox.closest('[data-confirmed-label]');
            expect(label?.dataset.confirmedState).toBe('confirmed');
        });
    });

    it('filters visible children by name and code', () => {
        const window = loadWindow();
        window.__test.setChildrenData([
            { id: 'pc:1', first_name: 'Alice', last_name: 'Smith', security_code: '1234', source: 'planning_center' },
            { id: 'pc:2', first_name: 'Bob', last_name: 'Jones', security_code: '5678', source: 'planning_center' }
        ]);
        window.__test.setSearchQuery('ali');
        expect(window.__test.getVisibleChildren().map((c) => c.id)).toEqual(['pc:1']);
        window.__test.setSearchQuery('5678');
        expect(window.__test.getVisibleChildren().map((c) => c.id)).toEqual(['pc:2']);
        window.__test.setSearchQuery('');
        expect(window.__test.getVisibleChildren()).toHaveLength(2);
    });

    it('renders no-matching message for empty search results', () => {
        const window = loadWindow({ html: '<!doctype html><html><body><ul id="children-list"></ul></body></html>' });
        window.__test.setChildrenData([]);
        window.__test.setDom();
        window.__test.setSearchQuery('zzz');
        expect(window.document.getElementById('children-list').innerHTML).toContain('No matching children');
    });
});
