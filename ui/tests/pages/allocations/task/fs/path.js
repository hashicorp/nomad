import { create, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/fs/:path'),

  tempTitle: text('h1.title'),
});
