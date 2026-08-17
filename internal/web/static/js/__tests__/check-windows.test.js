import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/js/check-windows.js');
const script = fs.readFileSync(scriptPath, 'utf8');

const fixtureHtml = `<!doctype html>
<html>
<body>
    <div id="page-status" class="hidden"></div>
</body>
</html>`;

function response(status, body) {
    return {
        status,
        ok: status >= 200 && status < 300,
        json: async () => body,
        text: async () => (typeof body === 'string' ? body : JSON.stringify(body))
    };
}

function loadWindow(fetchImpl, extra = {}) {
    const dom = new JSDOM(fixtureHtml, {
        runScripts: 'dangerously',
        url: 'http://localhost/'
    });

    dom.window.fetch = fetchImpl;
    if (extra.confirm !== undefined) {
        dom.window.confirm = extra.confirm;
    }
    dom.window.setPageStatus = (message, tone) => {
        const el = dom.window.document.getElementById('page-status');
        el.textContent = message;
        el.dataset.tone = tone || 'info';
    };
    dom.window.clearPageStatus = () => {
        const el = dom.window.document.getElementById('page-status');
        el.textContent = '';
        delete el.dataset.tone;
    };

    dom.window.eval(script);
    return dom.window;
}

function windowsResponse() {
    return response(200, [
        {
            id: 5,
            event_id: 1,
            start_day_of_week: 1,
            start_time: '09:00',
            end_day_of_week: 1,
            end_time: '12:00',
            timezone: 'America/Chicago'
        }
    ]);
}

function tick() {
    return new Promise(resolve => setTimeout(resolve, 0));
}

describe('check-windows shared modal', () => {
    it('injects the modal markup hidden on load', () => {
        const window = loadWindow(async () => {
            throw new Error('no fetch expected');
        });

        const modal = window.document.getElementById('window-modal');
        expect(modal).not.toBeNull();
        expect(modal.classList.contains('hidden')).toBe(true);
        expect(window.document.getElementById('window-form')).not.toBeNull();
    });

    it('opens the modal and renders windows for the event', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return windowsResponse();
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        const modal = window.document.getElementById('window-modal');
        expect(modal.classList.contains('hidden')).toBe(false);
        const listHtml = window.document.getElementById('modal-windows-list').innerHTML;
        expect(listHtml).toContain('Monday 9:00 AM - Monday 12:00 PM (America/Chicago)');
        expect(listHtml).toContain('Edit');
        expect(listHtml).toContain('Delete');
        expect(window.document.getElementById('modal-title').textContent).toContain('Kids Check-in');
    });

    it('prefills the edit form time fields in 12-hour format', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return windowsResponse();
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        window.openEditWindow(5);

        expect(window.document.getElementById('start-time').value).toBe('9:00');
        expect(window.document.getElementById('start-time-ampm').value).toBe('AM');
        expect(window.document.getElementById('end-time').value).toBe('12:00');
        expect(window.document.getElementById('end-time-ampm').value).toBe('PM');
        expect(window.document.getElementById('window-form').classList.contains('hidden')).toBe(false);
    });

    it('shows an empty-state message when an event has no check windows', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        const listHtml = window.document.getElementById('modal-windows-list').innerHTML;
        expect(listHtml).toContain('No check windows configured');
    });

    it('automatically opens the add window form when an event has no check windows', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        const form = window.document.getElementById('window-form');
        expect(form.classList.contains('hidden')).toBe(false);
        expect(window.document.getElementById('window-id').value).toBe('');
        expect(window.document.getElementById('modal-title').textContent).toContain('Add Check Window');
    });

    it('shows the Add Window button when an event has check windows', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return windowsResponse();
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        expect(window.document.getElementById('add-window-button').style.display).not.toBe('none');
    });

    it('shows validation errors when submitting an empty add form', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        window.openAddWindow();

        const form = window.document.getElementById('window-form');
        form.dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true }));

        expect(window.document.getElementById('start-time-error').classList.contains('hidden')).toBe(false);
        expect(window.document.getElementById('start-time-error').textContent).toBe('Start time is required');
        expect(window.document.getElementById('end-time-error').classList.contains('hidden')).toBe(false);
    });

    it('posts a new check window on valid form submit and closes the modal', async () => {
        let posted = null;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/check-windows') && options.method === 'POST') {
                posted = JSON.parse(options.body);
                return response(201, {});
            }
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        window.openAddWindow();

        window.document.getElementById('start-time').value = '9:00';
        window.document.getElementById('start-time-ampm').value = 'AM';
        window.document.getElementById('end-time').value = '12:00';
        window.document.getElementById('end-time-ampm').value = 'PM';
        window.document.getElementById('window-form').dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true }));
        await tick();

        expect(posted).toEqual({
            start_day_of_week: 7,
            start_time: '09:00',
            end_day_of_week: 7,
            end_time: '12:00',
            timezone: 'America/Chicago'
        });
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);
        const status = window.document.getElementById('page-status');
        expect(status.textContent).toBe('Check window created successfully');
    });

    it('deletes a window after confirmation and closes the modal', async () => {
        let deleted = null;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/check-windows') && options.method === 'DELETE') {
                deleted = url;
                return response(204, null);
            }
            if (url.includes('/check-windows')) {
                return windowsResponse();
            }
            return response(200, []);
        }, { confirm: () => true });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        await window.deleteWindow(5);

        expect(deleted).toContain('/v1/admin/events/1/check-windows/5');
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);
        const status = window.document.getElementById('page-status');
        expect(status.textContent).toBe('Check window deleted successfully');
    });

    it('does not delete when confirmation is declined', async () => {
        let deleted = false;
        const window = loadWindow(async (url, options = {}) => {
            if (url.includes('/check-windows') && options.method === 'DELETE') {
                deleted = true;
                return response(204, null);
            }
            if (url.includes('/check-windows')) {
                return windowsResponse();
            }
            return response(200, []);
        }, { confirm: () => false });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        await window.deleteWindow(5);

        expect(deleted).toBe(false);
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(false);
    });

    it('closes the modal on Escape', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return response(200, []);
            }
            return response(200, []);
        });

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(false);

        window.document.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(true);
    });
});