import {
  attribute,
  create,
  collection,
  clickable,
  fillable,
  text,
  isPresent,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import twoStepButton from 'nomad-ui/tests/pages/components/two-step-button';
import notification from 'nomad-ui/tests/pages/components/notification';

export default create({
  visit: visitable('/clients/:id'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  title: text('[data-test-title]'),

  statusLight: collection('[data-test-node-status]', {
    id: attribute('data-test-node-status'),
    text: text(),
  }),

  statusDefinition: text('[data-test-status-definition]'),
  statusDecorationClass: attribute('class', '[data-test-status-definition] .status-text'),
  addressDefinition: text('[data-test-address-definition]'),
  drainingDefinition: text('[data-test-draining]'),
  eligibilityDefinition: text('[data-test-eligibility]'),
  datacenterDefinition: text('[data-test-datacenter-definition]'),

  resourceCharts: collection('[data-test-primary-metric]', {
    name: text('[data-test-primary-metric-title]'),
    chartClass: attribute('class', '[data-test-percentage-chart] progress'),
  }),

  ...allocations(),

  allocationFilter: {
    preemptions: clickable('[data-test-filter-preemptions]'),
    all: clickable('[data-test-filter-all]'),
    preemptionsCount: text('[data-test-filter-preemptions]'),
    allCount: text('[data-test-filter-all]'),
  },

  attributesTable: isPresent('[data-test-attributes]'),
  metaTable: isPresent('[data-test-meta]'),
  emptyMetaMessage: isPresent('[data-test-empty-meta-message]'),

  metaAttributes: collection('[data-test-meta] [data-test-attributes-section]', {
    key: text('[data-test-key]'),
    value: text('[data-test-value]'),
  }),

  error: {
    isShown: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  hasEvents: isPresent('[data-test-client-events]'),
  events: collection('[data-test-client-event]', {
    time: text('[data-test-client-event-time]'),
    subsystem: text('[data-test-client-event-subsystem]'),
    message: text('[data-test-client-event-message]'),
  }),

  driverHeads: collection('[data-test-driver-status] [data-test-accordion-head]', {
    name: text('[data-test-name]'),
    detected: text('[data-test-detected]'),
    lastUpdated: text('[data-test-last-updated]'),
    healthIsShown: isPresent('[data-test-health]'),
    health: text('[data-test-health]'),
    healthClass: attribute('class', '[data-test-health] .color-swatch'),

    toggle: clickable('[data-test-accordion-toggle]'),
  }),

  driverBodies: collection('[data-test-driver-status] [data-test-accordion-body]', {
    description: text('[data-test-health-description]'),
    descriptionIsShown: isPresent('[data-test-health-description]'),
    attributesAreShown: isPresent('[data-test-driver-attributes]'),
  }),

  drainDetails: {
    scope: '[data-test-drain-details]',
    durationIsPresent: isPresent('[data-test-duration]'),
    duration: text('[data-test-duration]'),
    durationTooltip: attribute('aria-label', '[data-test-duration]'),
    deadline: text('[data-test-deadline]'),
    deadlineTooltip: attribute('aria-label', '[data-test-deadline]'),
    forceDrainText: text('[data-test-force-drain-text]'),
    drainSystemJobsText: text('[data-test-drain-system-jobs-text]'),

    completeCount: text('[data-test-complete-count]'),
    migratingCount: text('[data-test-migrating-count]'),
    remainingCount: text('[data-test-remaining-count]'),
    status: text('[data-test-status]'),
    force: twoStepButton('[data-test-force]'),
  },

  drainPopover: {
    toggle: clickable('[data-test-drain-popover] [data-test-popover-trigger]'),
    toggleDeadline: clickable('[data-test-drain-deadline-toggle]'),
    deadlineOptions: {
      open: clickable('[data-test-drain-deadline-toggle]'),
      options: collection('data-test-drain-deadline-options]', {
        label: text(),
        choose: clickable(),
      }),
    },

    setCustomDeadline: fillable('[data-test-drain-custom-deadline]'),
    toggleForceDrain: clickable(),
    toggleSystemJobs: clickable(),

    setDeadline(label) {
      this.deadlineOptions.open();
      this.deadlineOptions.options
        .toArray()
        .findBy('label', label)
        .choose();
    },
  },

  stopDrain: twoStepButton('[data-test-stop-drain]'),
  stopDrainIsPresent: isPresent('[data-test-stop-drain]'),

  toggleEligibility: clickable('[data-test-eligibility-toggle]'),

  eligibilityError: notification('[data-test-eligibility-error]'),
  stopDrainError: notification('[data-test-stop-drain-error]'),
  drainError: notification('[data-test-drain-error]'),
  drainStoppedNotification: notification('[data-test-drain-stopped-notification]'),
  drainUpdatedNotification: notification('[data-test-drain-updated-notification]'),
  drainCompleteNotification: notification('[data-test-drain-complete-notification]'),
});
