import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const scriptPath = path.resolve(process.cwd(), 'internal/web/static/pages/admin/location-groups.js');
const script = fs.readFileSync(scriptPath, 'utf8');
const exposeInternals = `
window.__test = {
    setLocationGroups: (groups) => { locationGroups = groups; },
    renderLocationGroups: () => renderLocationGroups(),
    createLocationGroup,
    updateLocationGroup,
    deleteLocationGroup
};
`;

const fixtureHtml = `<!doctype html>
    <html>
        <body>
            <div id="page-status" class="hidden"></div>
            <div id="location-groups-list"></div>
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

    dom.window.eval(`${script}\n${exposeInternals}`);
    return dom.window;
}

describe('admin/location-groups', () => {
    it('createLocationGroup posts to /v1/admin/location_groups', async () => {
        const calls = [];
        const fetchImpl = async (url, opts = {}) => {
            calls.push({ url, opts });
            return { ok: true, status: 201, json: async () => ({}), text: async () => '' };
        };
        const window = loadWindow(fetchImpl);

        await window.__test.createLocationGroup('K-3');

        const post = calls.find(c => c.url === '/v1/admin/location_groups');
        expect(post).toBeTruthy();
        expect(post.opts.method).toBe('POST');
        expect(JSON.parse(post.opts.body)).toEqual({ name: 'K-3' });
    });

    it('deleteLocationGroup surfaces in-use message', async () => {
        const window = loadWindow(async () => ({
            ok: false,
            status: 400,
            json: async () => ({ message: 'location group is in use' }),
            text: async () => ''
        }));

        await expect(window.__test.deleteLocationGroup(1)).rejects.toThrow('location group is in use');
    });

    it('explains why a group cannot be deleted when it is in use', async () => {
        const window = loadWindow(async (url) => {
            if (url === '/v1/admin/location_groups/1') {
                return {
                    ok: false,
                    status: 400,
                    json: async () => ({ sorry: 'location group is in use' }),
                    text: async () => ''
                };
            }
            return defaultFetch();
        });
        window.__test.setLocationGroups([{ id: 1, name: 'K-3' }]);
        window.__test.renderLocationGroups();

        window.document.querySelector('.delete-group-button[data-group-id="1"]').click();
        await new Promise(resolve => setTimeout(resolve, 0));
        await new Promise(resolve => setTimeout(resolve, 0));

        expect(window.document.getElementById('page-status').textContent)
            .toContain('Cannot delete "K-3": it is assigned to one or more locations or events');
    });

    it('renderLocationGroups renders rows with inputs', () => {
        const window = loadWindow();
        window.__test.setLocationGroups([{ id: 1, name: 'K-3' }]);

        window.__test.renderLocationGroups();

        const list = window.document.getElementById('location-groups-list');
        expect(list.innerHTML).toContain('K-3');
        expect(list.querySelector('.save-group-button')).not.toBeNull();
        expect(list.querySelector('.delete-group-button')).not.toBeNull();
    });
});