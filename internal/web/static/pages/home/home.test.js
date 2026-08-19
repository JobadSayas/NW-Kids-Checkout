import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/home/home.js');
const script = fs.readFileSync(scriptPath, 'utf8');

const fallbackCards = `
    <a href="/v1/checkins/checkouts?location_group_name=Kids%20Jr&checked_out_after=-31m" class="grade-card">Kids Jr</a>
    <a href="/v1/checkins/checkouts?location_group_name=Kids&checked_out_after=-31m" class="grade-card">1st - 6th Grade</a>
`;

const fixtureHtml = `<!doctype html>
    <html>
        <body>
            <div id="department-cards">${fallbackCards}</div>
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

    dom.window.eval(script);
    return dom.window;
}

describe('home', () => {
    it('renders a card per location group', async () => {
        const window = loadWindow(async () => ({
            ok: true,
            status: 200,
            json: async () => [{ id: 1, name: 'Kids Jr' }, { id: 2, name: 'Kids' }],
            text: async () => ''
        }));

        await window.__test.loadLocationGroups();

        const container = window.document.getElementById('department-cards');
        const links = container.querySelectorAll('a');
        expect(links.length).toBe(2);
        expect(links[0].getAttribute('href')).toBe('/v1/checkins/checkouts?location_group_name=Kids%20Jr&checked_out_after=-31m');
        expect(links[0].textContent).toContain('Kids Jr');
        expect(links[1].getAttribute('href')).toBe('/v1/checkins/checkouts?location_group_name=Kids&checked_out_after=-31m');
        expect(links[1].textContent).toContain('Kids');
    });

    it('keeps fallback cards when the fetch fails', async () => {
        const window = loadWindow(async () => {
            throw new Error('network down');
        });

        await window.__test.loadLocationGroups();

        expect(window.document.getElementById('department-cards').innerHTML).toContain('Kids Jr');
    });

    it('keeps fallback cards when no groups exist', async () => {
        const window = loadWindow();

        await window.__test.loadLocationGroups();

        expect(window.document.getElementById('department-cards').innerHTML).toContain('1st - 6th Grade');
    });

    it('departmentCardMarkup escapes the group name', () => {
        const window = loadWindow();
        const html = window.__test.departmentCardMarkup({ id: 1, name: '<Kids & Co>' });
        expect(html).toContain('&lt;Kids &amp; Co&gt;');
    });
});