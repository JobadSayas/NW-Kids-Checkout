import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';

const checkoutsPath = path.resolve(process.cwd(), 'internal/web/static/pages/checkoutsv1/checkouts.js');
const previewPath = path.resolve(process.cwd(), 'internal/web/dev-assets/preview.js');
const checkoutsScript = fs.readFileSync(checkoutsPath, 'utf8');
const previewScript = fs.readFileSync(previewPath, 'utf8');
const expose = `
window.__test = {
    getChildrenData: () => childrenData,
    getApiCallBlocks: () => API_CALL_BLOCKS
};
`;

function loadWindow() {
    const dom = new JSDOM('<!doctype html><html><body></body></html>', {
        runScripts: 'dangerously'
    });
    dom.window.fetch = async () => ({
        ok: true,
        json: async () => [],
        text: async () => ''
    });
    dom.window.setInterval = () => 0;
    dom.window.morphdom = () => {};
    dom.window.eval(`${checkoutsScript}\n${expose}\n${previewScript}`);
    return dom.window;
}

describe('checkoutsv1/preview', () => {
    it('loadPreviewData seeds demo checkouts and blocks auto-refresh', () => {
        const window = loadWindow();
        window.loadPreviewData();

        expect(window.__test.getApiCallBlocks().fetchChildrenData).toBe(true);

        const data = window.__test.getChildrenData();
        expect(data).toHaveLength(4);
        expect(data.map((child) => child.planning_center_id))
            .toEqual(['demo-0', 'demo-4', 'demo-8', 'demo-c']);
        expect(data[0].checked_out_confirmed_at).toBeNull();
        expect(data[3].checked_out_confirmed_at).toBe('2024-01-01T00:00:00Z');
    });
});