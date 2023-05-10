// @ts-check

/**
 * @typedef {('running' | 'pending' | 'failed' | 'lost' | 'complete' | 'unplaced')[]} AllocationClientStatuses
 * @typedef {Object.<string, AllocationClientStatuses>} JobAllocStatuses
 */

/**
 * @type {JobAllocStatuses}
 */
export const jobAllocStatuses = {
  service: ['running', 'pending', 'failed', 'lost', 'unplaced'],
  system: ['running', 'pending', 'failed', 'lost', 'unplaced'],
  batch: ['running', 'pending', 'failed', 'lost', 'complete', 'unplaced'],
};

export const jobTypes = ['service', 'system', 'batch'];
