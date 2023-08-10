/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  create,
  collection,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import twoStepButton from 'nomad-ui/tests/pages/components/two-step-button';
import LifecycleChart from 'nomad-ui/tests/pages/components/lifecycle-chart';

export default create({
  visit: visitable('/allocations/:id'),

  title: text('[data-test-title]'),

  stop: twoStepButton('[data-test-stop]'),
  restart: twoStepButton('[data-test-restart]'),
  restartAll: twoStepButton('[data-test-restart-all]'),

  execButton: {
    scope: '[data-test-exec-button]',
  },

  details: {
    scope: '[data-test-allocation-details]',

    job: text('[data-test-job-link]'),
    visitJob: clickable('[data-test-job-link]'),

    client: text('[data-test-client-link]'),
    visitClient: clickable('[data-test-client-link]'),
  },

  resourceCharts: collection('[data-test-primary-metric]', {
    name: text('[data-test-primary-metric-title]'),
    chartClass: attribute('class', '[data-test-percentage-chart] progress'),
  }),

  resourceEmptyMessage: text('[data-test-resource-error-headline]'),

  lifecycleChart: LifecycleChart,

  tasks: collection('[data-test-task-row]', {
    name: text('[data-test-name]'),
    state: text('[data-test-state]'),
    message: text('[data-test-message]'),
    time: text('[data-test-time]'),
    volumes: text('[data-test-volumes]'),

    hasCpuMetrics: isPresent('[data-test-cpu] .inline-chart progress'),
    hasMemoryMetrics: isPresent('[data-test-mem] .inline-chart progress'),
    hasUnhealthyDriver: isPresent('[data-test-icon="unhealthy-driver"]'),
    hasProxyTag: isPresent('[data-test-proxy-tag]'),

    clickLink: clickable('[data-test-name] a'),
    clickRow: clickable('[data-test-name]'),
  }),

  firstUnhealthyTask() {
    return this.tasks.toArray().findBy('hasUnhealthyDriver');
  },

  hasRescheduleEvents: isPresent('[data-test-reschedule-events]'),

  isEmpty: isPresent('[data-test-empty-tasks-list]'),

  wasPreempted: isPresent('[data-test-was-preempted]'),
  preempter: {
    scope: '[data-test-was-preempted]',

    status: text('[data-test-allocation-status]'),
    name: text('[data-test-allocation-name]'),
    priority: text('[data-test-job-priority]'),
    reservedCPU: text('[data-test-allocation-cpu]'),
    reservedMemory: text('[data-test-allocation-memory]'),

    visit: clickable('[data-test-allocation-id]'),
    visitJob: clickable('[data-test-job-link]'),
    visitClient: clickable('[data-test-client-link]'),
  },

  preempted: isPresent('[data-test-preemptions]'),
  ...allocations(
    '[data-test-preemptions] [data-test-allocation]',
    'preemptions'
  ),

  ports: collection('[data-test-allocation-port]', {
    name: text('[data-test-allocation-port-name]'),
    address: text('[data-test-allocation-port-address]'),
    to: text('[data-test-allocation-port-to]'),
  }),

  services: collection('[data-test-service]', {
    name: text('[data-test-service-name]'),
    port: text('[data-test-service-port]'),
    tags: text('[data-test-service-tags]'),
    onUpdate: text('[data-test-service-onupdate]'),
    connect: text('[data-test-service-connect]'),
    upstreams: text('[data-test-service-upstreams]'),
  }),

  error: {
    isShown: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  inlineError: {
    isShown: isPresent('[data-test-inline-error]'),
    title: text('[data-test-inline-error-title]'),
    message: text('[data-test-inline-error-body]'),
    dismiss: clickable('[data-test-inline-error-close]'),
  },
});
