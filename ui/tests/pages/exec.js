import { clickable, collection, create, hasClass, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/exec/:job'),

  taskGroups: collection('[data-test-task-group]', {
    click: clickable('[data-test-task-group-name]'),
    name: text('[data-test-task-group-name]'),

    chevron: {
      scope: '.icon',
      isDown: hasClass('icon-is-chevron-down'),
      isRight: hasClass('icon-is-chevron-right'),
    },

    tasks: collection('[data-test-task]', {
      name: text(),
    }),
  }),
});
