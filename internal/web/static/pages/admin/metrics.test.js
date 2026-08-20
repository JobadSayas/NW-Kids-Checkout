import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/metrics.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const exposeInternals = `
window.__test = { renderMetrics, loadMetrics };
`;

const fixtureHtml = `<!doctype html>
    <html>
        <body>
            <div id="page-status" class="hidden"></div>
            <div id="metrics-error" class="hidden"></div>
            <div id="metrics-days"><option value="7">7</option><option value="14" selected>14</option><option value="30">30</option></div>
            <div id="metrics-body"></div>
        </body>
    </html>`;

const defaultFetch = async () => ({
    ok: true,
    status: 200,
    json: async () => ({ days: 14, daily: [] }),
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

describe('admin/metrics', () => {
    it('renderMetrics renders rows and empty state', () => {
        const window = loadWindow();
        window.__test.renderMetrics({
            days: 14,
            daily: [
                { date: '2026-08-18', event_name: 'Kids', called: 5, confirmed: 4, unconfirmed: 1, avg_confirm_minutes: 3.5, manual_count: 2 },
            ],
        });
        let html = window.document.getElementById('metrics-body').innerHTML;
        expect(html).toContain('Kids');
        expect(html).toContain('3.5');
        expect(html).toContain('2026-08-18');

        window.__test.renderMetrics({ days: 14, daily: [] });
        html = window.document.getElementById('metrics-body').innerHTML;
        expect(html).toContain('No data yet.');
    });

    it('loadMetrics builds URL with days param', async () => {
        const calls = [];
        const fetchImpl = async (url) => {
            calls.push(url);
            return { ok: true, status: 200, json: async () => ({ days: 7, daily: [] }), text: async () => '' };
        };
        const window = loadWindow(fetchImpl);
        const data = await window.__test.loadMetrics(7);
        expect(calls[0]).toContain('days=7');
        expect(data.days).toBe(7);
    });
});