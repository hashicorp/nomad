import { attribute, collection, clickable, hasClass, isPresent, text } from 'ember-cli-page-object';

const allocationRect = {
  select: clickable(),
  width: attribute('width', '> rect'),
  height: attribute('height', '> rect'),
  isActive: hasClass('is-active'),
  isSelected: hasClass('is-selected'),
  running: hasClass('running'),
  failed: hasClass('failed'),
  pending: hasClass('pending'),
};

export default scope => ({
  scope,

  label: text('[data-test-label]'),
  labelIsPresent: isPresent('[data-test-label]'),
  statusIcon: attribute('class', '[data-test-status-icon] .icon'),
  statusIconLabel: attribute('aria-label', '[data-test-status-icon]'),

  selectNode: clickable('[data-test-node-background]'),
  nodeIsInteractive: hasClass('is-interactive', '[data-test-node-background]'),
  nodeIsSelected: hasClass('is-selected', '[data-test-node-background]'),

  memoryRects: collection('[data-test-memory-rect]', {
    ...allocationRect,
    id: attribute('data-test-memory-rect'),
  }),
  cpuRects: collection('[data-test-cpu-rect]', {
    ...allocationRect,
    id: attribute('data-test-cpu-rect'),
  }),

  emptyMessage: text('[data-test-empty-message]'),
  isEmpty: hasClass('is-empty'),
});
