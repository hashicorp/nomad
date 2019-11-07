import { create, isPresent, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/logs'),

  hasTaskLog: isPresent('[data-test-task-log]'),
});
