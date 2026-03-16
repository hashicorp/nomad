/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

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
  batch: ['running', 'pending', 'complete', 'failed', 'lost', 'unplaced'],
  sysbatch: ['running', 'pending', 'complete', 'failed', 'lost', 'unplaced'],
};

export const jobTypes = ['service', 'system', 'batch', 'sysbatch'];
