import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/manual-checkins/manual-checkins.js');
const script = fs.readFileSync(scriptPath, 'utf8');

function loadWindow({ url = 'http://localhost/' } = {}) {
    const html = `<!doctype html>
        <html>
            <body>
                <div id="page-status" class="hidden"></div>
                <table>
                    <tbody id="manual-checkins-body"></tbody>
                </table>
                <button id="open-manual-checkin"></button>
                <div id="manual-checkin-modal" class="hidden" aria-hidden="true">
                    <form id="manual-checkin-form">
                        <input id="manual-first-name" name="first" value="" />
                        <input id="manual-last-name" name="last" value="" />
                        <input id="manual-immediate-checkout" type="checkbox" />
                        <button id="manual-checkin-submit" type="submit">Save</button>
                    </form>
                    <button data-modal-close></button>
                    <div id="manual-checkin-error" class="hidden"></div>
                </div>
            </body>
        </html>`;

    const dom = new JSDOM(html, {
        runScripts: 'dangerously',
        url
    });

    dom.window.fetch = async () => ({
        ok: true,
        status: 200,
        json: async () => [],
        text: async () => ''
    });
    dom.window.setInterval = () => 0;
    if (!dom.window.AbortController) {
        dom.window.AbortController = class {
            constructor() {
                this.signal = {};
            }
            abort() {
                this.signal.aborted = true;
            }
        };
    }

    dom.window.eval(script);
    return dom.window;
}

describe('manual-checkins', () => {
    it('builds query params with defaults', () => {
        const window = loadWindow();
        const query = window.buildManualCheckinsQuery();
        const params = new URLSearchParams(query);
        expect(params.get('checked_out_after')).toBe('-12h');
        expect(params.get('include_unchecked')).toBe('true');
        expect(params.get('sort')).toBe('created');
        expect(params.get('limit')).toBe(null);
    });

    it('builds query params from search string', () => {
        const window = loadWindow({
            url: 'http://localhost/?checked_out_after=-2h&limit=50&include_unchecked=false'
        });
        const query = window.buildManualCheckinsQuery();
        const params = new URLSearchParams(query);
        expect(params.get('checked_out_after')).toBe('-2h');
        expect(params.get('include_unchecked')).toBe('false');
        expect(params.get('sort')).toBe('created');
        expect(params.get('limit')).toBe('50');
    });

    it('formats checked out timestamps', () => {
        const window = loadWindow();
        expect(window.formatCheckedOutAt('')).toBe('—');
        expect(window.formatCheckedOutAt('not-a-date')).toBe('—');
        const value = '2024-01-01T00:00:00Z';
        const expected = new window.Date(value).toLocaleString();
        expect(window.formatCheckedOutAt(value)).toBe(expected);
    });

    it('renders a zero-state message', () => {
        const window = loadWindow();
        window.renderManualCheckins([]);
        const body = window.document.getElementById('manual-checkins-body');
        expect(body.innerHTML).toContain('No manual check-ins found.');
    });

    it('renders manual check-in rows with status and actions', () => {
        const window = loadWindow();
        window.renderManualCheckins([
            {
                first_name: 'Ada',
                last_name: 'Lovelace',
                public_id: 'a1',
                checked_out_at: '2024-01-01T00:00:00Z'
            },
            {
                first_name: 'Grace',
                last_name: 'Hopper',
                public_id: 'b2',
                checked_out_at: ''
            }
        ]);

        const rows = window.document.querySelectorAll('#manual-checkins-body tr');
        expect(rows.length).toBe(2);

        const firstStatus = rows[0].querySelector('td[data-label="Status"] span');
        expect(firstStatus.textContent).toBe('Checked out');
        expect(firstStatus.className).toContain('bg-emerald-100');

        const firstButton = rows[0].querySelector('button');
        expect(firstButton.textContent).toBe('Undo Checkout');
        expect(firstButton.dataset.publicId).toBe('a1');
        expect(firstButton.dataset.checkedOut).toBe('true');

        const secondStatus = rows[1].querySelector('td[data-label="Status"] span');
        expect(secondStatus.textContent).toBe('Pending');
        expect(secondStatus.className).toContain('bg-amber-100');

        const secondButton = rows[1].querySelector('button');
        expect(secondButton.textContent).toBe('Check Out');
        expect(secondButton.dataset.publicId).toBe('b2');
        expect(secondButton.dataset.checkedOut).toBe('false');
    });

    it('toggles the manual check-in modal and resets form state', () => {
        const window = loadWindow();
        const modal = window.document.getElementById('manual-checkin-modal');
        const firstName = window.document.getElementById('manual-first-name');
        const lastName = window.document.getElementById('manual-last-name');
        const error = window.document.getElementById('manual-checkin-error');

        window.toggleManualCheckinModal(true);
        expect(modal.classList.contains('hidden')).toBe(false);
        expect(modal.getAttribute('aria-hidden')).toBe('false');

        firstName.value = 'Ada';
        lastName.value = 'Lovelace';
        window.setManualCheckinError('Required');
        expect(error.classList.contains('hidden')).toBe(false);

        window.toggleManualCheckinModal(false);
        expect(modal.classList.contains('hidden')).toBe(true);
        expect(modal.getAttribute('aria-hidden')).toBe('true');
        expect(firstName.value).toBe('');
        expect(lastName.value).toBe('');
        expect(error.classList.contains('hidden')).toBe(true);
    });
});
