/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  collection,
  hasClass,
  isPresent,
  text,
} from 'ember-cli-page-object';
import { getter } from 'ember-cli-page-object/macros';

import toggle from 'nomad-ui/tests/pages/components/toggle';

export default {
  scope: '[data-test-task-group-recommendations]',

  slug: {
    jobName: text('[data-test-job-name]'),
    groupName: text('[data-test-task-group-name]'),
  },

  namespace: text('[data-test-namespace]'),

  copyButton: {
    scope: '[data-test-copy-button]',
    clipboardText: attribute('data-clipboard-text', 'button'),
  },

  totalsTable: totalsTableComponent('[data-test-group-totals]'),

  narrative: text('[data-test-narrative]'),

  togglesTable: {
    scope: '[data-test-toggles-table]',

    toggleAllIsPresent: isPresent('[data-test-toggle-all]'),
    toggleAllCPU: toggle('[data-test-tasks-head] [data-test-cpu-toggle]'),
    toggleAllMemory: toggle('[data-test-tasks-head] [data-test-memory-toggle]'),

    tasks: collection('[data-test-task-toggles]', {
      name: text('[data-test-name]'),
      cpu: toggle('[data-test-cpu-toggle]'),
      memory: toggle('[data-test-memory-toggle]'),

      isActive: hasClass('active'),
    }),
  },

  activeTask: {
    scope: '[data-test-active-task]',

    name: text('[data-test-task-name]'),
    totalsTable: totalsTableComponent(''),

    charts: collection('[data-test-chart-for]', {
      resource: text('text.resource'),
    }),

    cpuChart: resourceChartComponent('[data-test-chart-for=CPU]'),
    memoryChart: resourceChartComponent('[data-test-chart-for=MemoryMB]'),
  },

  acceptButton: {
    scope: '[data-test-accept]',
    isDisabled: attribute('disabled'),
  },

  dismissButton: {
    scope: '[data-test-dismiss]',
  },
};

function totalsTableCell(scope) {
  return {
    scope,
    isIncrease: hasClass('increase'),
    isDecrease: hasClass('decrease'),
    isNeutral: getter(function () {
      return !this.isIncrease && !this.isDecrease;
    }),
  };
}

function totalsTableComponent(scope) {
  return {
    scope,

    current: {
      scope: '[data-test-current]',
      cpu: totalsTableCell('[data-test-cpu]'),
      memory: totalsTableCell('[data-test-memory]'),
    },

    recommended: {
      scope: '[data-test-recommended]',
      cpu: totalsTableCell('[data-test-cpu]'),
      memory: totalsTableCell('[data-test-memory]'),
    },

    unitDiff: {
      cpu: text('[data-test-cpu-unit-diff]'),
      memory: text('[data-test-memory-unit-diff]'),
    },

    percentDiff: {
      cpu: text('[data-test-cpu-percent-diff]'),
      memory: text('[data-test-memory-percent-diff]'),
    },
  };
}

function resourceChartComponent(scope) {
  return {
    scope,

    isIncrease: hasClass('increase'),
    isDecrease: hasClass('decrease'),
    isDisabled: hasClass('disabled'),
  };
}
