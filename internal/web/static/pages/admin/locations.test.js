import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/locations.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const checkWindowsScriptPath = path.resolve(process.cwd(), 'internal/web/static/js/check-windows.js');
const checkWindowsScript = fs.readFileSync(checkWindowsScriptPath, 'utf8');
const exposeInternals = `
window.__test = {
    setEvents: (events) => { eventsById = new Map(events.map(event => [Number(event.id), event])); },
    setLocationGroups: (groups) => { locationGroups = groups; },
    renderLocations: () => renderLocations()
};
`;

const fixtureHtml = `<!doctype html>
    <html>
        <body>
            <div id="page-status" class="hidden"></div>
            <table>
                <tbody id="locations-body"></tbody>
            </table>
        </body>
    </html>`;

const defaultFetch = async () => ({
    ok: true,
    status: 200,
    json: async () => [],
    text: async () => ''
});

function loadWindow(fetchImpl = defaultFetch) {
    const dom = new JSDOM(fixtureHtml, {
        runScripts: 'dangerously',
        url: 'http://localhost/'
    });

    dom.window.fetch = fetchImpl;
    dom.window.setInterval = () => 0;

    dom.window.eval(checkWindowsScript);
    dom.window.eval(`${script}\n${exposeInternals}`);
    return dom.window;
}

describe('admin/locations', () => {
    it('renders the auto-fetch toggle inside a content-sized inline-flex label', () => {
        const window = loadWindow();
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: true, location_group_id: null }]);

        window.__test.renderLocations();

        const toggle = window.document.querySelector('.event-auto-fetch-toggle');
        expect(toggle).not.toBeNull();

        const label = toggle.closest('label');
        expect(label.classList.contains('inline-flex')).toBe(true);
        expect(label.classList.contains('flex')).toBe(false);
    });

    it('renders a Check Windows button for each event row', () => {
        const window = loadWindow();
        window.__test.setEvents([
            { id: 1, name: 'Kids Check-in', auto_fetch: true, location_group_id: null },
            { id: 2, name: 'Sunday Service', auto_fetch: false, location_group_id: null }
        ]);

        window.__test.renderLocations();

        const buttons = window.document.querySelectorAll('[onclick^="openCheckWindowsModal"]');
        expect(buttons.length).toBe(2);
        expect(buttons[0].getAttribute('onclick')).toBe('openCheckWindowsModal(1, "Kids Check-in")');
        expect(buttons[1].getAttribute('onclick')).toBe('openCheckWindowsModal(2, "Sunday Service")');
    });

    it('renders a Check Windows button for an event even when it has no locations', () => {
        const window = loadWindow();
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);
        window.__test.setLocationGroups([]);

        window.__test.renderLocations();

        const button = window.document.querySelector('[onclick^="openCheckWindowsModal"]');
        expect(button).not.toBeNull();
    });

    it('opens the check windows modal and renders windows for the event', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return {
                    ok: true,
                    status: 200,
                    json: async () => [
                        { id: 5, event_id: 1, start_day_of_week: 1, start_time: '09:00', end_day_of_week: 1, end_time: '12:00', timezone: 'America/Chicago' }
                    ],
                    text: async () => ''
                };
            }
            return defaultFetch();
        });
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        expect(window.document.getElementById('window-modal').classList.contains('hidden')).toBe(false);
        const listHtml = window.document.getElementById('modal-windows-list').innerHTML;
        expect(listHtml).toContain('Monday 9:00 AM - Monday 12:00 PM (America/Chicago)');
        expect(listHtml).toContain('Edit');
        expect(listHtml).toContain('Delete');
    });

    it('prefills the edit form time fields in 12-hour format', async () => {
        const window = loadWindow(async url => {
            if (url.includes('/check-windows')) {
                return {
                    ok: true,
                    status: 200,
                    json: async () => [
                        { id: 5, event_id: 1, start_day_of_week: 1, start_time: '14:30', end_day_of_week: 1, end_time: '00:15', timezone: 'America/Chicago' }
                    ],
                    text: async () => ''
                };
            }
            return defaultFetch();
        });
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);

        await window.openCheckWindowsModal(1, 'Kids Check-in');
        window.openEditWindow(5);

        expect(window.document.getElementById('start-time').value).toBe('2:30');
        expect(window.document.getElementById('start-time-ampm').value).toBe('PM');
        expect(window.document.getElementById('end-time').value).toBe('12:15');
        expect(window.document.getElementById('end-time-ampm').value).toBe('AM');
    });

    it('shows an empty-state message when an event has no check windows', async () => {
        const window = loadWindow();
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);

        await window.openCheckWindowsModal(1, 'Kids Check-in');

        const listHtml = window.document.getElementById('modal-windows-list').innerHTML;
        expect(listHtml).toContain('No check windows configured');
    });

    it('hides the Add Window button while the add form is shown', () => {
        const window = loadWindow();
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);

        window.openAddWindow();

        expect(window.document.getElementById('add-window-button').style.display).toBe('none');
        expect(window.document.getElementById('window-form').classList.contains('hidden')).toBe(false);
    });

    it('shows the Add Window button again after cancelling the form', () => {
        const window = loadWindow();
        window.__test.setEvents([{ id: 1, name: 'Kids Check-in', auto_fetch: false, location_group_id: null }]);

        window.openAddWindow();
        window.cancelForm();

        expect(window.document.getElementById('add-window-button').style.display).toBe('');
        expect(window.document.getElementById('window-form').classList.contains('hidden')).toBe(true);
    });
});