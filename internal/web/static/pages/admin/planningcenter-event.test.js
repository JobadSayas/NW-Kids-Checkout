import { describe, it, expect, vi } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/planningcenter-event.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const checkWindowsScriptPath = path.resolve(process.cwd(), 'internal/web/static/js/check-windows.js');
const checkWindowsScript = fs.readFileSync(checkWindowsScriptPath, 'utf8');

const fixtureHtml = `<!doctype html>
<html>
<body>
    <div id="page-status" class="hidden"></div>
    <button id="add-event-button" type="button">Add event to system</button>
    <p id="event-added-status" class="hidden">This event has already been added to the system. Click "Refresh event data" to get the most up-to-date locations for the event.</p>
    <span id="event-id-label"></span>
    <span id="event-name-label"></span>
    <table>
        <tbody id="locations-body"></tbody>
    </table>
    <div id="auto-fetch-modal" class="hidden">
        <input type="checkbox" id="auto-fetch-toggle">
        <div id="auto-fetch-error" class="hidden"></div>
        <button id="auto-fetch-continue" type="button">Continue</button>
    </div>
</body>
</html>`;

function loadWindow(fetchImpl) {
    const dom = new JSDOM(fixtureHtml, {
        runScripts: 'dangerously',
        url: 'http://localhost/admin/planningcenter/events/999999'
    });

    dom.window.fetch = fetchImpl;
    dom.window.eval(checkWindowsScript);
    dom.window.eval(script);

    return dom.window;
}

function tick() {
    return new Promise(resolve => setTimeout(resolve, 0));
}

function response(status, body) {
    return {
        status,
        ok: status >= 200 && status < 300,
        json: async () => body,
        text: async () => (typeof body === 'string' ? body : JSON.stringify(body))
    };
}

const notFoundResponse = {
    status: 404,
    ok: false,
    json: async () => null,
    text: async () => 'event not found'
};

describe('admin/planningcenter-event', () => {
    it('keeps the "already been added" message hidden when the event is not in the system', async () => {
        let lookupCalls = 0;
        const window = loadWindow(async url => {
            if (url.includes('/lookup')) {
                lookupCalls++;
                return notFoundResponse;
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        expect(lookupCalls).toBe(1);
        const status = window.document.getElementById('event-added-status');
        expect(status.classList.contains('hidden')).toBe(true);
    });

    it('shows the "already been added" message when the event pre-exists', async () => {
        const existingEvent = { id: 1, name: 'Weekend Experience', planning_center_id: '999999' };
        const window = loadWindow(async url => {
            if (url.includes('/lookup')) {
                return response(200, existingEvent);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        const status = window.document.getElementById('event-added-status');
        expect(status.classList.contains('hidden')).toBe(false);
    });

    it('does not claim an event was "already added" when the user just added it', async () => {
        let syncCalls = 0;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                syncCalls++;
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999' });
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        const status = window.document.getElementById('event-added-status');
        expect(status.classList.contains('hidden')).toBe(true);

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();

        expect(syncCalls).toBe(1);
        expect(status.classList.contains('hidden')).toBe(true);
    });

    it('opens the auto-fetch modal (not check windows) after a successful refresh', async () => {
        let syncCalls = 0;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                syncCalls++;
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: false });
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        const autoFetchModal = window.document.getElementById('auto-fetch-modal');
        expect(autoFetchModal).not.toBeNull();
        expect(autoFetchModal.classList.contains('hidden')).toBe(true);

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        expect(syncCalls).toBe(1);
        expect(autoFetchModal.classList.contains('hidden')).toBe(false);
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);
    });

    it('pre-fills the auto-fetch toggle from the DB value after refresh', async () => {
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: true });
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        expect(window.document.getElementById('auto-fetch-toggle').checked).toBe(true);
    });

    it('defaults the auto-fetch toggle to on for a newly added event', async () => {
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: false });
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        expect(window.document.getElementById('auto-fetch-toggle').checked).toBe(true);
    });

    it('persists the chosen auto-fetch value on Continue, then opens the check windows modal', async () => {
        let patchCalls = 0;
        let patchPayload = null;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: false });
            }
            if (url.includes('/v1/admin/events/1') && options.method === 'PATCH') {
                patchCalls++;
                patchPayload = JSON.parse(options.body);
                return response(200, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: patchPayload.auto_fetch });
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        window.document.getElementById('auto-fetch-toggle').checked = true;
        window.document.getElementById('auto-fetch-continue').click();
        await tick();
        await tick();

        expect(patchCalls).toBe(1);
        expect(patchPayload).toEqual({ auto_fetch: true });
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(false);
    });

    it('does not PATCH when the auto-fetch value is unchanged, and still opens check windows', async () => {
        let patchCalls = 0;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: true });
            }
            if (url.includes('/v1/admin/events/1') && options.method === 'PATCH') {
                patchCalls++;
                return response(200, {});
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        expect(window.document.getElementById('auto-fetch-modal').classList.contains('hidden')).toBe(false);
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);

        window.document.getElementById('auto-fetch-continue').click();
        await tick();
        await tick();

        expect(patchCalls).toBe(0);
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(false);
    });

    it('keeps the auto-fetch modal open and shows an error when persisting fails', async () => {
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/lookup')) {
                return notFoundResponse;
            }
            if (url.includes('/v1/admin/events') && options.method === 'POST') {
                return response(201, { id: 1, name: 'Weekend Experience', planning_center_id: '999999', auto_fetch: false });
            }
            if (url.includes('/v1/admin/events/1') && options.method === 'PATCH') {
                return { status: 500, ok: false, json: async () => null, text: async () => 'server error' };
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            if (url.includes('/locations')) {
                return response(200, []);
            }
            throw new Error(`unexpected fetch: ${url}`);
        });
        await tick();

        window.document.getElementById('add-event-button').click();
        await tick();
        await tick();
        await tick();

        window.document.getElementById('auto-fetch-toggle').checked = true;
        window.document.getElementById('auto-fetch-continue').click();
        await tick();
        await tick();

        const modal = window.document.getElementById('auto-fetch-modal');
        expect(modal.classList.contains('hidden')).toBe(false);
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);
        const errorEl = window.document.getElementById('auto-fetch-error');
        expect(errorEl.classList.contains('hidden')).toBe(false);
        expect(errorEl.textContent).toContain('Failed');
    });
});