import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/fetcher-status.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const exposeInternals = `
window.__test = { renderEvents, loadEvents, formatAge };
`;

const fixtureHtml = `<!doctype html>
    <html>
        <body>
            <div id="page-status" class="hidden"></div>
            <div id="fetcher-status-error" class="hidden"></div>
            <div id="fetcher-status-body"></div>
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
    dom.window.eval(exposeInternals);
    return dom.window;
}

describe('admin/fetcher-status', () => {
    it('renders events with correct status', async () => {
        const window = loadWindow();
        window.__test.renderEvents([
            { id: 1, name: 'Kids Check-in', auto_fetch: true, last_checked_out_time: new Date(Date.now() - 10 * 1000).toISOString() },
            { id: 2, name: 'Sunday Service', auto_fetch: false, last_checked_out_time: new Date(Date.now() - 10 * 60 * 1000).toISOString() },
            { id: 3, name: 'Stale Event', auto_fetch: true, last_checked_out_time: new Date(Date.now() - 20 * 60 * 1000).toISOString() }
        ]);

        const html = window.document.getElementById('fetcher-status-body').innerHTML;
        expect(html).toContain('Kids Check-in');
        expect(html).toContain('ok');
        expect(html).toContain('Sunday Service');
        expect(html).toContain('ok');
        expect(html).toContain('Stale Event');
        expect(html).toContain('stale');
    });

    it('renders "never" if last_checked_out_time is null', () => {
        const window = loadWindow();
        window.__test.renderEvents([
            { id: 1, name: 'Never Event', auto_fetch: true, last_checked_out_time: null }
        ]);

        const html = window.document.getElementById('fetcher-status-body').innerHTML;
        expect(html).toContain('Never Event');
        expect(html).toContain('never');
    });

    it('formats age correctly', () => {
        const window = loadWindow();
        expect(window.formatAge(500)).toBe('0s');
        expect(window.formatAge(60000)).toBe('1m');
        expect(window.formatAge(3600000)).toBe('1h');
    });
});