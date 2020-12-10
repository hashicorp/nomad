import {
  clickable,
  create,
  collection,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import error from 'nomad-ui/tests/pages/components/error';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';
import stepperInput from 'nomad-ui/tests/pages/components/stepper-input';
import LifecycleChart from 'nomad-ui/tests/pages/components/lifecycle-chart';

export default create({
  pageSize: 25,

  visit: visitable('/jobs/:id/:name'),

  search: fillable('.search-box input'),

  countStepper: stepperInput('[data-test-task-group-count-stepper]'),

  tasksCount: text('[data-test-task-group-tasks]'),
  cpu: text('[data-test-task-group-cpu]'),
  mem: text('[data-test-task-group-mem]'),
  disk: text('[data-test-task-group-disk]'),

  ...allocations(),

  isEmpty: isPresent('[data-test-empty-allocations-list]'),

  lifecycleChart: LifecycleChart,

  hasVolumes: isPresent('[data-test-volumes]'),
  volumes: collection('[data-test-volumes] [data-test-volume]', {
    name: text('[data-test-volume-name]'),
    type: text('[data-test-volume-type]'),
    source: text('[data-test-volume-source]'),
    permissions: text('[data-test-volume-permissions]'),
  }),

  hasScaleEvents: isPresent('[data-test-scale-events]'),
  scaleEvents: collection('[data-test-scale-events] [data-test-accordion-head]', {
    error: isPresent('[data-test-error]'),
    time: text('[data-test-time]'),
    count: text('[data-test-count]'),
    countIcon: { scope: '[data-test-count-icon]' },
    message: text('[data-test-message]'),

    isToggleable: isPresent('[data-test-accordion-toggle]:not(.is-invisible)'),
    toggle: clickable('[data-test-accordion-toggle]'),
  }),

  scaleEventBodies: collection('[data-test-scale-events] [data-test-accordion-body]', {
    meta: text(),
  }),

  hasScalingTimeline: isPresent('[data-test-scaling-timeline]'),
  scalingAnnotations: collection('[data-test-scaling-timeline] [data-test-annotation]', {
    open: clickable('button'),
  }),

  error: error(),

  emptyState: {
    headline: text('[data-test-empty-allocations-list-headline]'),
  },

  pageSizeSelect: pageSizeSelect(),
});
