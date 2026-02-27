import {createRequire} from 'node:module';
import {describe, expect, it} from 'vitest';

const require = createRequire(import.meta.url);
const {sortLocationsByPlanningCenterHierarchy} = require('../location-sort.js');

describe('sortLocationsByPlanningCenterHierarchy', () => {
    it('orders roots by event ID then planning center ID, children by name', () => {
        const items = [
            {
                planning_center_id: 'root-b',
                planning_center_parent_id: null,
                name: 'Root B',
                event_id: 2
            },
            {
                planning_center_id: 'root-a',
                planning_center_parent_id: '',
                name: 'Root A',
                event_id: 1
            },
            {
                planning_center_id: 'child-b2',
                planning_center_parent_id: 'root-b',
                name: 'Beta',
                event_id: 2
            },
            {
                planning_center_id: 'child-a2',
                planning_center_parent_id: 'root-a',
                name: 'Zulu',
                event_id: 1
            },
            {
                planning_center_id: 'child-a1',
                planning_center_parent_id: 'root-a',
                name: 'Alpha',
                event_id: 1
            },
            {
                planning_center_id: 'child-b1',
                planning_center_parent_id: 'root-b',
                name: 'Alpha',
                event_id: 2
            },
            {
                planning_center_id: 'orphan',
                planning_center_parent_id: 'missing',
                name: 'Orphan',
                event_id: 1
            },
            {
                planning_center_id: 'no-event',
                planning_center_parent_id: null,
                name: 'No Event',
                event_id: null
            }
        ];

        const result = sortLocationsByPlanningCenterHierarchy(items).map(item => item.planning_center_id);

        expect(result).toEqual([
            'orphan',
            'root-a',
            'child-a1',
            'child-a2',
            'root-b',
            'child-b1',
            'child-b2',
            'no-event'
        ]);
    });

    it('orders roots by planning center ID when event IDs match', () => {
        const items = [
            {
                planning_center_id: 'Root-C',
                planning_center_parent_id: null,
                name: 'Gamma',
                event_id: 1
            },
            {
                planning_center_id: 'root-a',
                planning_center_parent_id: null,
                name: 'Alpha',
                event_id: 1
            },
            {
                planning_center_id: 'root-b',
                planning_center_parent_id: null,
                name: 'Beta',
                event_id: 1
            }
        ];

        const result = sortLocationsByPlanningCenterHierarchy(items).map(item => item.planning_center_id);

        expect(result).toEqual(['root-a', 'root-b', 'Root-C']);
    });
});
