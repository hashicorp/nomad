import { attribute, collection, clickable, isPresent, text } from 'ember-cli-page-object';

export default function(selector = '[data-test-allocation]', propKey = 'allocations') {
  return {
    [propKey]: collection(selector, {
      id: attribute('data-test-allocation'),
      shortId: text('[data-test-short-id]'),
      createTime: text('[data-test-create-time]'),
      modifyTime: text('[data-test-modify-time]'),
      status: text('[data-test-client-status]'),
      job: text('[data-test-job]'),
      taskGroup: text('[data-test-task-group]'),
      client: text('[data-test-client]'),
      jobVersion: text('[data-test-job-version]'),
      cpu: text('[data-test-cpu]'),
      cpuTooltip: attribute('aria-label', '[data-test-cpu] .tooltip'),
      mem: text('[data-test-mem]'),
      memTooltip: attribute('aria-label', '[data-test-mem] .tooltip'),
      rescheduled: isPresent('[data-test-indicators] [data-test-icon="reschedule"]'),

      visit: clickable('[data-test-short-id] a'),
      visitJob: clickable('[data-test-job]'),
      visitClient: clickable('[data-test-client] a'),
    }),

    allocationFor(id) {
      return this.allocations.toArray().find(allocation => allocation.id === id);
    },
  };
}
