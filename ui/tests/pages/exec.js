import { clickable, collection, create, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/exec/:job'),

  taskGroups: collection('[data-test-task-group]', {
    click: clickable('[data-test-task-group-name]'),
    name: text('[data-test-task-group-name]'),

    tasks: collection('[data-test-task]', {
      name: text(),
    }),
  }),
});
