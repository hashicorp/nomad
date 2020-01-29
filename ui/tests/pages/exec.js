import { clickable, collection, create, hasClass, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/exec/:job'),

  header: {
    region: { scope: '[data-test-region]' },
    namespace: { scope: '[data-test-namespace]' },
    job: text('[data-test-job]'),
  },

  taskGroups: collection('[data-test-task-group]', {
    click: clickable('[data-test-task-group-name]'),
    name: text('[data-test-task-group-name]'),

    chevron: {
      scope: '.toggle-button .icon',
      isDown: hasClass('icon-is-chevron-down'),
      isRight: hasClass('icon-is-chevron-right'),
    },

    tasks: collection('[data-test-task]', {
      name: text(),
    }),
  }),
});
