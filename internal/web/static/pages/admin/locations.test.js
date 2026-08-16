import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/locations.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const exposeInternals = `
window.__test = {
    setEvents: (events) => { eventsById = new Map(events.map(event => [Number(event.id), event])); },
    setLocationGroups: (groups) => { locationGroups = groups; },
    renderLocations: () => renderLocations()
};
`;

function loadWindow() {
    const html = `<!doctype html>
        <html>
            <body>
                <div id="page-status" class="hidden"></div>
                <table>
                    <tbody id="locations-body"></tbody>
                </table>
            </body>
        </html>`;

    const dom = new JSDOM(html, {
        runScripts: 'dangerously',
        url: 'http://localhost/'
    });

    dom.window.fetch = async () => ({
        ok: true,
        status: 200,
        json: async () => [],
        text: async () => ''
    });
    dom.window.setInterval = () => 0;

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
});